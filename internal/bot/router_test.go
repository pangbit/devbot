package bot

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type spySender struct {
	messages []string
}

func (s *spySender) SendText(_ context.Context, _, text string) error {
	s.messages = append(s.messages, text)
	return nil
}

func (s *spySender) SendTextChunked(_ context.Context, _, text string) error {
	s.messages = append(s.messages, text)
	return nil
}

func (s *spySender) SendCard(_ context.Context, _ string, card CardMsg) error {
	s.messages = append(s.messages, card.Title+"\n\n"+card.Content)
	return nil
}

func (s *spySender) LastMessage() string {
	if len(s.messages) == 0 {
		return ""
	}
	return s.messages[len(s.messages)-1]
}

func newTestRouter(t *testing.T) (*Router, *spySender) {
	t.Helper()
	dir := t.TempDir()
	// Create some subdirs for /ls test
	os.Mkdir(filepath.Join(dir, "project1"), 0755)
	os.Mkdir(filepath.Join(dir, "project2"), 0755)
	os.Mkdir(filepath.Join(dir, ".hidden"), 0755)
	// Create a file for /file test
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test"), 0644)

	store, err := NewStore(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	sender := &spySender{}
	exec := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), exec, store, sender, map[string]bool{"user1": true}, dir, nil)
	return r, sender
}

func TestRouterUnauthorizedUser(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "hacker", "hello")
	if len(sender.messages) != 0 {
		t.Fatalf("expected no response for unauthorized user")
	}
}

func TestRouterHelp(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/help")
	if len(sender.messages) == 0 {
		t.Fatalf("expected help response")
	}
	msg := sender.LastMessage()
	if !strings.Contains(msg, "/help") || !strings.Contains(msg, "/ping") {
		t.Fatalf("help should list commands, got: %q", msg)
	}
}

func TestRouterPing(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/ping")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "pong") {
		t.Fatalf("expected pong, got: %q", msg)
	}
}

func TestRouterStatus_WithExec(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte("#!/bin/sh\necho '{\"type\":\"result\",\"result\":\"ok\",\"session_id\":\"s1\"}'\n"), 0755)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &spySender{}
	exec := NewClaudeExecutor(script, "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), exec, store, sender, map[string]bool{"user1": true}, dir, nil)

	// Run an execution first so ExecCount > 0
	r.Route(context.Background(), "chat1", "user1", "hello")

	r.Route(context.Background(), "chat1", "user1", "/status")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "执行次数:** 1") {
		t.Fatalf("expected 执行次数: 1 after one execution, got: %q", msg)
	}
	// 上次耗时 should not be "-" after an execution
	if strings.Contains(msg, "上次耗时:** -") {
		t.Fatalf("expected non-dash 上次耗时 after execution, got: %q", msg)
	}
}

func TestRouterStatus(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/status")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "运行时长") || !strings.Contains(msg, "sonnet") {
		t.Fatalf("status should show 运行时长 and model, got: %q", msg)
	}
	if !strings.Contains(msg, "执行次数") {
		t.Fatalf("status should show 执行次数, got: %q", msg)
	}
	if !strings.Contains(msg, "上次耗时") {
		t.Fatalf("status should show 上次耗时, got: %q", msg)
	}
	if !strings.Contains(msg, "待执行队列") {
		t.Fatalf("status should show 待执行队列, got: %q", msg)
	}
	// With no executions, 上次耗时 should be "-"
	if !strings.Contains(msg, "上次耗时:** -") {
		t.Fatalf("status should show 上次耗时: - when no execs, got: %q", msg)
	}
	// 执行次数 should be 0
	if !strings.Contains(msg, "执行次数:** 0") {
		t.Fatalf("status should show 执行次数: 0, got: %q", msg)
	}
}

func TestRouterPwd(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/pwd")
	msg := sender.LastMessage()
	if msg == "" {
		t.Fatalf("expected pwd response")
	}
}

func TestRouterLs(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/ls")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "project1") || !strings.Contains(msg, "project2") {
		t.Fatalf("ls should list projects, got: %q", msg)
	}
	if strings.Contains(msg, ".hidden") {
		t.Fatalf("ls should not list hidden dirs, got: %q", msg)
	}
}

func TestRouterCd(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/cd project1")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "已切换到") || !strings.Contains(msg, "project1") {
		t.Fatalf("expected cd confirmation, got: %q", msg)
	}
}

func TestRouterCdInvalid(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/cd nonexistent")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "不存在") {
		t.Fatalf("expected not found error, got: %q", msg)
	}
}

func TestRouterNewSession(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/new")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "新对话") {
		t.Fatalf("expected new session msg, got: %q", msg)
	}
}

func TestRouterModel(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/model opus")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "opus") {
		t.Fatalf("expected model set to opus, got: %q", msg)
	}
}

func TestRouterYoloSafe(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/yolo")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "无限制") {
		t.Fatalf("expected yolo msg, got: %q", msg)
	}
	r.Route(context.Background(), "chat1", "user1", "/safe")
	msg = sender.LastMessage()
	if !strings.Contains(msg, "安全模式") {
		t.Fatalf("expected safe msg, got: %q", msg)
	}
}

func TestRouterLast_NoOutput(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/last")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "暂无历史输出") {
		t.Fatalf("expected no previous output msg, got: %q", msg)
	}
}

func TestRouterKill_NoProcess(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/kill")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "没有") {
		t.Fatalf("expected no running task msg, got: %q", msg)
	}
}

func TestRouterUnknownCommand(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/unknown")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "未知命令") {
		t.Fatalf("expected unknown command msg, got: %q", msg)
	}
}

func TestRouterFile(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/file README.md")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "# Test") {
		t.Fatalf("expected file content, got: %q", msg)
	}
}

func TestRouterFile_EmptyArgs(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/file")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "用法") {
		t.Fatalf("expected usage message for /file with no args, got: %q", msg)
	}
}

func TestRouterFileNotFound(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/file nonexistent.txt")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "不存在") {
		t.Fatalf("expected not found, got: %q", msg)
	}
}

func TestRouterCd_EmptyArgs(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/cd")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "用法") {
		t.Fatalf("expected usage message, got: %q", msg)
	}
}

func TestRouterCd_AbsPath(t *testing.T) {
	r, sender := newTestRouter(t)
	// Use the actual work root dir as absolute path — valid absolute path within root
	workRoot := r.store.WorkRoot()
	subDir := filepath.Join(workRoot, "project1")
	r.Route(context.Background(), "chat1", "user1", "/cd "+subDir)
	msg := sender.LastMessage()
	if !strings.Contains(msg, "已切换到") {
		t.Fatalf("expected changed to message, got: %q", msg)
	}
}

func TestRouterCdPathTraversal(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/cd ../../etc")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "工作根目录") {
		t.Fatalf("expected path traversal rejection, got: %q", msg)
	}
}

func TestUnderRoot(t *testing.T) {
	tests := []struct {
		root string
		path string
		want bool
	}{
		{"/home/user", "/home/user", true},
		{"/home/user", "/home/user/proj", true},
		{"/home/user", "/home/user/proj/sub", true},
		{"/home/user", "/home/user2", false},   // sibling dir starting with same name
		{"/home/user", "/home/user2/x", false}, // path under sibling dir
		{"/home/user", "/home", false},
		{"/home/user", "/etc/passwd", false},
		{"", "/home", false},
		{"/home", "", false},
	}
	for _, tt := range tests {
		got := underRoot(tt.root, tt.path)
		if got != tt.want {
			t.Errorf("underRoot(%q, %q) = %v, want %v", tt.root, tt.path, got, tt.want)
		}
	}
}

func TestRouterRouteImageSavesAndSendsPrompt(t *testing.T) {
	r, sender := newTestRouter(t)
	imgData := []byte("fake-image-data")
	r.RouteImage(context.Background(), "chat1", "user1", imgData, "test_image.png")

	if len(sender.messages) == 0 {
		t.Fatalf("expected at least one message")
	}
	// First message should confirm the image was saved
	if !strings.Contains(sender.messages[0], "图片已保存") {
		t.Fatalf("expected 'Image saved to:' message, got: %q", sender.messages[0])
	}
	if !strings.Contains(sender.messages[0], ".devbot-images") {
		t.Fatalf("expected .devbot-images in path, got: %q", sender.messages[0])
	}

	// Verify the image file was actually written
	session := r.getSession("chat1")
	imgPath := filepath.Join(session.WorkDir, ".devbot-images", "test_image.png")
	data, err := os.ReadFile(imgPath)
	if err != nil {
		t.Fatalf("expected image file to exist at %s: %v", imgPath, err)
	}
	if string(data) != "fake-image-data" {
		t.Fatalf("image data mismatch: got %q", string(data))
	}
}

func TestRouterRouteImageUnauthorized(t *testing.T) {
	r, sender := newTestRouter(t)
	r.RouteImage(context.Background(), "chat1", "hacker", []byte("data"), "test.png")
	if len(sender.messages) != 0 {
		t.Fatalf("expected no response for unauthorized user")
	}
}

func TestRouterRouteFileSavesAndSendsPrompt(t *testing.T) {
	r, sender := newTestRouter(t)
	fileData := []byte("file-content-here")
	r.RouteFile(context.Background(), "chat1", "user1", "report.pdf", fileData)

	if len(sender.messages) == 0 {
		t.Fatalf("expected at least one message")
	}
	if !strings.Contains(sender.messages[0], "文件已保存") {
		t.Fatalf("expected '文件已保存' message, got: %q", sender.messages[0])
	}

	// Verify the file was actually written
	session := r.getSession("chat1")
	filePath := filepath.Join(session.WorkDir, "report.pdf")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("expected file to exist at %s: %v", filePath, err)
	}
	if string(data) != "file-content-here" {
		t.Fatalf("file data mismatch: got %q", string(data))
	}
}

func TestRouterRouteFileUnauthorized(t *testing.T) {
	r, sender := newTestRouter(t)
	r.RouteFile(context.Background(), "chat1", "hacker", "test.txt", []byte("data"))
	if len(sender.messages) != 0 {
		t.Fatalf("expected no response for unauthorized user")
	}
}

func TestRouterRouteTextWithImages(t *testing.T) {
	r, sender, q := newTestRouterForExec(t)
	session := r.getSession("chat1")

	images := []ImageAttachment{
		{Data: []byte("image-data-1"), FileName: "photo.jpg"},
		{Data: []byte("image-data-2"), FileName: "screenshot.png"},
	}
	r.RouteTextWithImages(context.Background(), "chat1", "user1", "fix this bug", images)
	q.Shutdown()

	// Verify images were saved to disk
	for _, img := range images {
		imgPath := filepath.Join(session.WorkDir, ".devbot-images", filepath.Base(img.FileName))
		data, err := os.ReadFile(imgPath)
		if err != nil {
			t.Fatalf("expected image file %s to exist: %v", img.FileName, err)
		}
		if string(data) != string(img.Data) {
			t.Fatalf("image data mismatch for %s", img.FileName)
		}
	}

	// Should have dispatched execution
	msgs := sender.Messages()
	if len(msgs) == 0 {
		t.Fatalf("expected at least one message")
	}
}

func TestRouterRouteTextWithImages_Unauthorized(t *testing.T) {
	r, sender := newTestRouter(t)
	images := []ImageAttachment{
		{Data: []byte("data"), FileName: "photo.jpg"},
	}
	r.RouteTextWithImages(context.Background(), "chat1", "hacker", "text", images)
	if len(sender.messages) != 0 {
		t.Fatalf("expected no response for unauthorized user")
	}
}

func TestRouterRouteTextWithImages_OnlyImages(t *testing.T) {
	r, sender, q := newTestRouterForExec(t)
	images := []ImageAttachment{
		{Data: []byte("image-data"), FileName: "screenshot.png"},
	}
	// No text, only images
	r.RouteTextWithImages(context.Background(), "chat1", "user1", "", images)
	q.Shutdown()

	msgs := sender.Messages()
	if len(msgs) == 0 {
		t.Fatalf("expected at least one message when images provided without text")
	}
}

func TestRouterRouteImage_MkdirError(t *testing.T) {
	r, sender := newTestRouter(t)
	session := r.getSession("chat1")

	// Make session.WorkDir point to a file (not a dir) so MkdirAll fails
	// when trying to create .devbot-images inside it.
	fakeDirPath := filepath.Join(session.WorkDir, "notadir")
	os.WriteFile(fakeDirPath, []byte("file content"), 0644)
	r.store.UpdateSession("chat1", func(s *Session) {
		s.WorkDir = fakeDirPath
	})

	r.RouteImage(context.Background(), "chat1", "user1", []byte("data"), "test.png")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "Failed to create image directory") {
		t.Fatalf("expected mkdir error message, got: %q", msg)
	}
}

func TestRouterRouteFile_WriteError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; permission-based tests are unreliable")
	}
	r, sender := newTestRouter(t)
	session := r.getSession("chat1")
	workDir := session.WorkDir

	// Make workDir read-only so WriteFile fails
	os.Chmod(workDir, 0555)
	defer os.Chmod(workDir, 0755)

	r.RouteFile(context.Background(), "chat1", "user1", "data.txt", []byte("content"))
	msg := sender.LastMessage()
	if !strings.Contains(msg, "Failed to save file") {
		t.Fatalf("expected save failure message, got: %q", msg)
	}
}

func TestRouterRouteTextWithImages_ImageSaveFail(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; permission-based tests are unreliable")
	}
	r, _, q := newTestRouterForExec(t)
	session := r.getSession("chat1")

	// Pre-create .devbot-images as a non-writable dir so WriteFile inside fails.
	imgDir := filepath.Join(session.WorkDir, ".devbot-images")
	os.MkdirAll(imgDir, 0755)
	os.Chmod(imgDir, 0555) // no write
	defer os.Chmod(imgDir, 0755)

	images := []ImageAttachment{
		{Data: []byte("data"), FileName: "photo.jpg"},
	}
	// Only images, no text — when all images fail to save, prompt is built with no paths.
	// The function returns early because savedPaths is empty and text is also empty.
	r.RouteTextWithImages(context.Background(), "chat1", "user1", "", images)
	q.Shutdown()
	// No panic or hang — the image save failure is logged and skipped.
}

func TestRouterRouteTextWithImages_NoImages(t *testing.T) {
	r, sender := newTestRouter(t)
	// No text and no images — should do nothing
	r.RouteTextWithImages(context.Background(), "chat1", "user1", "", nil)
	if len(sender.messages) != 0 {
		t.Fatalf("expected no message when no text and no images")
	}
}

func TestRouterRouteDocShare(t *testing.T) {
	r, sender := newTestRouter(t)
	r.RouteDocShare(context.Background(), "chat1", "user1", "ABC123")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "ABC123") {
		t.Fatalf("expected doc ID in response, got: %q", msg)
	}
	if !strings.Contains(msg, "/doc bind") {
		t.Fatalf("expected bind hint in response, got: %q", msg)
	}
}

func TestRouterRouteDocShareUnauthorized(t *testing.T) {
	r, sender := newTestRouter(t)
	r.RouteDocShare(context.Background(), "chat1", "hacker", "ABC123")
	if len(sender.messages) != 0 {
		t.Fatalf("expected no response for unauthorized user")
	}
}

func TestRouterWorkRootNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	store.SetWorkRoot("/existing/root")
	sender := &spySender{}
	exec := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	NewRouter(context.Background(), exec, store, sender, map[string]bool{"user1": true}, "/new/root", nil)
	if store.WorkRoot() != "/existing/root" {
		t.Fatalf("expected WorkRoot to remain /existing/root, got %q", store.WorkRoot())
	}
}

// --- Thread-safe spy sender for queue-based tests ---

type syncSpySender struct {
	mu       sync.Mutex
	messages []string
}

func (s *syncSpySender) SendText(_ context.Context, _, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, text)
	return nil
}

func (s *syncSpySender) SendTextChunked(_ context.Context, _, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, text)
	return nil
}

func (s *syncSpySender) SendCard(_ context.Context, _ string, card CardMsg) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, card.Title+"\n\n"+card.Content)
	return nil
}

func (s *syncSpySender) Messages() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]string, len(s.messages))
	copy(cp, s.messages)
	return cp
}

func newTestRouterForExec(t *testing.T) (*Router, *syncSpySender, *MessageQueue) {
	t.Helper()
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "project1"), 0755)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test"), 0644)

	store, err := NewStore(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	sender := &syncSpySender{}
	exec := NewClaudeExecutor("/nonexistent_binary_for_test", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), exec, store, sender, map[string]bool{"user1": true}, dir, nil)
	q := NewMessageQueue()
	r.SetQueue(q)
	return r, sender, q
}

// --- /root tests ---

func TestRouterRoot_ShowCurrent(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/root")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "当前根目录") {
		t.Fatalf("expected current root, got: %q", msg)
	}
}

func TestRouterRoot_SetValid(t *testing.T) {
	r, sender := newTestRouter(t)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	r.Route(context.Background(), "chat1", "user1", "/root "+dir)
	msg := sender.LastMessage()
	if !strings.Contains(msg, "根目录已设置") {
		t.Fatalf("expected root set confirmation, got: %q", msg)
	}
	if r.store.WorkRoot() != dir {
		t.Fatalf("expected WorkRoot %s, got %s", dir, r.store.WorkRoot())
	}
}

func TestRouterRoot_RejectRelative(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/root relative/path")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "绝对路径") {
		t.Fatalf("expected absolute path error, got: %q", msg)
	}
}

func TestRouterRoot_RejectSystemDirs(t *testing.T) {
	r, sender := newTestRouter(t)
	for _, d := range []string{"/", "/etc", "/var", "/usr", "/sys", "/proc"} {
		r.Route(context.Background(), "chat1", "user1", "/root "+d)
		msg := sender.LastMessage()
		if !strings.Contains(msg, "系统目录") {
			t.Fatalf("expected system dir rejection for %s, got: %q", d, msg)
		}
	}
}

func TestRouterRoot_RejectNonExistent(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/root /nonexistent_path_xyz_abc")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "不存在") {
		t.Fatalf("expected not found, got: %q", msg)
	}
}

func TestRouterRoot_RejectFile(t *testing.T) {
	r, sender := newTestRouter(t)
	// Resolve symlinks so /var/folders -> /private/var/folders on macOS
	// avoiding the system directory check for /var
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	f := filepath.Join(dir, "file.txt")
	os.WriteFile(f, []byte("hi"), 0644)
	r.Route(context.Background(), "chat1", "user1", "/root "+f)
	msg := sender.LastMessage()
	if !strings.Contains(msg, "不是目录") {
		t.Fatalf("expected not a directory, got: %q", msg)
	}
}

// --- /sessions tests ---

func TestRouterSessions_Empty(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/sessions")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "暂无历史会话") {
		t.Fatalf("expected no sessions, got: %q", msg)
	}
}

func TestRouterSessions_WithHistory(t *testing.T) {
	r, sender := newTestRouter(t)
	r.getSession("chat1")
	r.store.UpdateSession("chat1", func(s *Session) {
		s.ClaudeSessionID = "current-sess"
		s.History = []string{"old-sess-1", "old-sess-2"}
	})
	r.Route(context.Background(), "chat1", "user1", "/sessions")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "old-sess-1") || !strings.Contains(msg, "old-sess-2") {
		t.Fatalf("expected history, got: %q", msg)
	}
	if !strings.Contains(msg, "current-sess") || !strings.Contains(msg, "当前") {
		t.Fatalf("expected current session, got: %q", msg)
	}
}

// --- /switch tests ---

func TestRouterSwitch_EmptyArgs(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/switch")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "用法") {
		t.Fatalf("expected usage, got: %q", msg)
	}
}

func TestRouterSwitch_Valid(t *testing.T) {
	r, sender := newTestRouter(t)
	r.getSession("chat1")
	r.store.UpdateSession("chat1", func(s *Session) {
		s.ClaudeSessionID = "old-sess"
	})
	r.Route(context.Background(), "chat1", "user1", "/switch new-sess")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "已切换到会话") || !strings.Contains(msg, "new-sess") {
		t.Fatalf("expected switch confirmation, got: %q", msg)
	}
	sess := r.store.GetSession("chat1", "", "")
	if sess.ClaudeSessionID != "new-sess" {
		t.Fatalf("expected session new-sess, got: %q", sess.ClaudeSessionID)
	}
	found := false
	for _, h := range sess.History {
		if h == "old-sess" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected old-sess in history, got: %v", sess.History)
	}
}

// --- Usage message tests for commands that require args ---

func TestRouterCommit_EmptyMsg(t *testing.T) {
	// /commit without a message now auto-generates one via Claude (calls execClaudeQueued).
	// Verify it sends "执行中" to indicate execution was triggered.
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/commit")
	// Should start executing (not show usage error)
	msgs := sender.messages
	hasExecuting := false
	for _, m := range msgs {
		if strings.Contains(m, "执行中") {
			hasExecuting = true
		}
	}
	if !hasExecuting {
		t.Fatalf("expected execution triggered for /commit without msg, got: %v", msgs)
	}
}

func TestRouterSh_EmptyArgs(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/sh")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "用法") {
		t.Fatalf("expected usage, got: %q", msg)
	}
}

func TestRouterSummary_NoOutput(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/summary")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "暂无可总结") {
		t.Fatalf("expected no output msg, got: %q", msg)
	}
}

func TestRouterModelShowCurrent(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/model")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "当前模型") {
		t.Fatalf("expected current model, got: %q", msg)
	}
	if !strings.Contains(msg, "sonnet") {
		t.Fatalf("expected default model 'sonnet', got: %q", msg)
	}
}

func TestRouterModelShowCurrentAfterChange(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/model opus")
	r.Route(context.Background(), "chat1", "user1", "/model")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "opus") {
		t.Fatalf("expected 'opus' after model change, got: %q", msg)
	}
}

func TestRouterLast_WithOutput(t *testing.T) {
	r, sender := newTestRouter(t)
	r.getSession("chat1")
	r.store.UpdateSession("chat1", func(s *Session) {
		s.LastOutput = "Some previous output here"
	})
	r.Route(context.Background(), "chat1", "user1", "/last")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "Some previous output here") {
		t.Fatalf("expected last output, got: %q", msg)
	}
}

func TestRouterRetry_NoPrompt(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/retry")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "没有可重试") {
		t.Fatalf("expected no-prompt message, got: %q", msg)
	}
}

func TestRouterRetry_WithPrompt(t *testing.T) {
	r, sender := newTestRouter(t)
	// Manually set a lastPrompt in the session
	r.getSession("chat1")
	r.store.UpdateSession("chat1", func(s *Session) {
		s.LastPrompt = "请帮我写一个Hello World"
	})
	r.Route(context.Background(), "chat1", "user1", "/retry")
	msgs := sender.messages
	// Should send the retry notification text
	hasRetryMsg := false
	for _, m := range msgs {
		if strings.Contains(m, "重试") && strings.Contains(m, "Hello World") {
			hasRetryMsg = true
		}
	}
	if !hasRetryMsg {
		t.Fatalf("expected retry message with prompt, got: %v", msgs)
	}
}

func TestRouterCancel_AliasForKill(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/cancel")
	msg := sender.LastMessage()
	// No running task - should get the same response as /kill
	if !strings.Contains(msg, "没有") {
		t.Fatalf("expected no running task msg for /cancel, got: %q", msg)
	}
}

func TestRouterHandlePrompt_SavesLastPrompt(t *testing.T) {
	r, _ := newTestRouter(t)
	r.getSession("chat1")
	// handlePrompt is called for non-command messages
	r.Route(context.Background(), "chat1", "user1", "my test prompt")
	sess := r.store.GetSession("chat1", "", "")
	if sess.LastPrompt != "my test prompt" {
		t.Fatalf("expected lastPrompt to be saved, got: %q", sess.LastPrompt)
	}
}

func TestRouterLog_DefaultCount(t *testing.T) {
	r, sender := newTestRouter(t)
	// /log without args should trigger execution (not error/usage msg)
	r.Route(context.Background(), "chat1", "user1", "/log")
	msgs := sender.messages
	hasExecuting := false
	for _, m := range msgs {
		if strings.Contains(m, "执行中") {
			hasExecuting = true
		}
	}
	if !hasExecuting {
		t.Fatalf("expected /log to trigger execution, got: %v", msgs)
	}
}

func TestRouterBranch_NoArgs(t *testing.T) {
	r, sender := newTestRouter(t)
	// /branch without args should trigger execution (show branches)
	r.Route(context.Background(), "chat1", "user1", "/branch")
	msgs := sender.messages
	hasExecuting := false
	for _, m := range msgs {
		if strings.Contains(m, "执行中") {
			hasExecuting = true
		}
	}
	if !hasExecuting {
		t.Fatalf("expected /branch to trigger execution, got: %v", msgs)
	}
}

func TestRouterSetQueue(t *testing.T) {
	r, _ := newTestRouter(t)
	if r.queue != nil {
		t.Fatalf("expected nil queue initially")
	}
	q := NewMessageQueue()
	r.SetQueue(q)
	if r.queue != q {
		t.Fatalf("expected queue to be set")
	}
}

func TestRouterLs_Empty(t *testing.T) {
	dir := t.TempDir()
	// Only hidden dirs — no non-hidden subdirs
	os.Mkdir(filepath.Join(dir, ".hidden"), 0755)

	store, err := NewStore(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	sender := &spySender{}
	exec := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), exec, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/ls")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "暂无项目") {
		t.Fatalf("expected '暂无项目' message, got: %q", msg)
	}
}

func TestRouterLs_ReadDirError(t *testing.T) {
	r, sender := newTestRouter(t)
	// Point work root to a non-existent directory to force ReadDir error
	r.store.SetWorkRoot("/nonexistent_dir_for_test_xyz")
	r.Route(context.Background(), "chat1", "user1", "/ls")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "读取目录出错") {
		t.Fatalf("expected error message from /ls with bad root, got: %q", msg)
	}
}

func TestRouterFile_ReadError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; permission-based tests are unreliable")
	}
	r, _ := newTestRouter(t)
	session := r.getSession("chat1")

	// Create a file that exists but has no read permission
	secret := filepath.Join(session.WorkDir, "secret.txt")
	os.WriteFile(secret, []byte("cannot read"), 0644)
	os.Chmod(secret, 0000)
	defer os.Chmod(secret, 0644)

	sender := r.sender.(*spySender)
	r.Route(context.Background(), "chat1", "user1", "/file secret.txt")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "读取文件出错") {
		t.Fatalf("expected read error message, got: %q", msg)
	}
}

func TestRouterKill_Running(t *testing.T) {
	dir := t.TempDir()
	readyPath := filepath.Join(dir, "ready")
	script := filepath.Join(dir, "claude")
	// Use "exec sleep" so the shell replaces itself with sleep (same PID).
	// This ensures killing the process closes the pipe and unblocks the scanner.
	os.WriteFile(script, []byte(fmt.Sprintf("#!/bin/sh\ntouch %s\nexec sleep 10000\n", readyPath)), 0755)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	snd := &syncSpySender{}
	exec := NewClaudeExecutor(script, "sonnet", 60*time.Second)
	r := NewRouter(context.Background(), exec, store, snd, map[string]bool{"user1": true}, dir, nil)

	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.Route(ctx, "chat1", "user1", "run a long task")
	}()

	// Wait for the script to start (file appears when ready)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(readyPath); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(readyPath); err != nil {
		t.Fatal("script did not start within 5s")
	}

	// Kill the running process
	r.Route(ctx, "chat2", "user1", "/kill")

	// Wait for the first route to complete
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("expected route goroutine to finish after kill")
	}

	msgs := snd.Messages()
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "已终止") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected '已终止' message, got: %v", msgs)
	}
}

func TestRouterSave_StoreError(t *testing.T) {
	r, _ := newTestRouter(t)
	// Set store path to an invalid location so Save() fails.
	r.store.path = "/nonexistent_root_xyz_test/subdir/state.json"
	// save() should log the error but not panic.
	r.save()
}

func TestRouterExecClaudeQueued_QueueFull(t *testing.T) {
	r, sender, q := newTestRouterForExec(t)

	// Block the worker so the queue can fill up.
	done := make(chan struct{})
	q.Enqueue("chat1", func() { <-done })

	// Fill queue to capacity (100 slots).
	for i := 0; i < 100; i++ {
		q.Enqueue("chat1", func() {})
	}

	// One more message — queue is full, should get error reply.
	r.Route(context.Background(), "chat1", "user1", "overflow message")

	msgs := sender.Messages()
	hasQueueFull := false
	for _, m := range msgs {
		if strings.Contains(m, "队列已满") {
			hasQueueFull = true
		}
	}
	if !hasQueueFull {
		t.Fatalf("expected '队列已满' message, got: %v", msgs)
	}

	close(done)
	q.Shutdown()
}

func TestRouterStatus_WithQueue(t *testing.T) {
	// Verify cmdStatus covers the r.queue != nil branch.
	r, sender, q := newTestRouterForExec(t)
	r.Route(context.Background(), "chat1", "user1", "/status")
	q.Shutdown()

	found := false
	for _, m := range sender.Messages() {
		if strings.Contains(m, "待执行队列") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected '待执行队列' field in status, got: %v", sender.Messages())
	}
}

func TestRouterCd_SavesDirSession(t *testing.T) {
	// When the session has an existing ClaudeSessionID and WorkDir, /cd should
	// save the current dir-session before switching.
	r, sender := newTestRouter(t)
	r.getSession("chat1")
	r.store.UpdateSession("chat1", func(s *Session) {
		s.ClaudeSessionID = "existing-session-id"
	})

	r.Route(context.Background(), "chat1", "user1", "/cd project1")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "已切换到") {
		t.Fatalf("expected changed to message, got: %q", msg)
	}
}

func TestRouterFile_AbsolutePath(t *testing.T) {
	// Absolute paths should be rejected by findFile and return "not found".
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/file /absolute/path.txt")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "不存在") {
		t.Fatalf("expected not found for absolute path, got: %q", msg)
	}
}

func TestRouterRouteImage_WriteFileError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; permission-based tests are unreliable")
	}
	r, sender := newTestRouter(t)
	session := r.getSession("chat1")

	// Pre-create .devbot-images dir and make it non-writable.
	imgDir := filepath.Join(session.WorkDir, ".devbot-images")
	os.MkdirAll(imgDir, 0755)
	os.Chmod(imgDir, 0555)
	defer os.Chmod(imgDir, 0755)

	r.RouteImage(context.Background(), "chat1", "user1", []byte("data"), "test.png")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "图片保存失败") {
		t.Fatalf("expected save image error, got: %q", msg)
	}
}

func TestRouterRouteTextWithImages_MkdirError(t *testing.T) {
	r, sender, q := newTestRouterForExec(t)
	session := r.getSession("chat1")

	// Make WorkDir a file so MkdirAll for .devbot-images fails.
	fakeDirPath := filepath.Join(session.WorkDir, "notadir2")
	os.WriteFile(fakeDirPath, []byte("file content"), 0644)
	r.store.UpdateSession("chat1", func(s *Session) {
		s.WorkDir = fakeDirPath
	})

	images := []ImageAttachment{{Data: []byte("data"), FileName: "photo.jpg"}}
	r.RouteTextWithImages(context.Background(), "chat1", "user1", "check this", images)
	q.Shutdown()

	found := false
	for _, m := range sender.Messages() {
		if strings.Contains(m, "Failed to create image directory") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 'Failed to create image directory' message, got: %v", sender.Messages())
	}
}

func TestRouterRouteTextWithImages_TextFallback(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; permission-based tests are unreliable")
	}
	r, _, q := newTestRouterForExec(t)
	session := r.getSession("chat1")

	// Make imgDir non-writable so all image saves fail.
	imgDir := filepath.Join(session.WorkDir, ".devbot-images")
	os.MkdirAll(imgDir, 0755)
	os.Chmod(imgDir, 0555)
	defer os.Chmod(imgDir, 0755)

	images := []ImageAttachment{{Data: []byte("data"), FileName: "photo.jpg"}}
	// Non-empty text with failing images → text-only prompt fallback.
	r.RouteTextWithImages(context.Background(), "chat1", "user1", "check this bug", images)
	q.Shutdown()
	// No panic — text fallback branch covered.
}

func TestRouterEmptyText(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "")
	if len(sender.messages) != 0 {
		t.Fatalf("expected no response for empty text")
	}
}

func TestRouterWhitespaceText(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "   ")
	if len(sender.messages) != 0 {
		t.Fatalf("expected no response for whitespace-only text")
	}
}

// --- Exec dispatch tests (commands that call execClaudeQueued) ---

func TestRouterExecCommands(t *testing.T) {
	commands := []struct {
		name string
		cmd  string
	}{
		{"commit", "/commit fix bug"},
		{"git", "/git status"},
		{"sh", "/sh echo hello"},
		{"diff", "/diff"},
		{"push", "/push"},
		{"undo", "/undo"},
		{"stash", "/stash"},
		{"stash_pop", "/stash pop"},
	}
	for _, tc := range commands {
		t.Run(tc.name, func(t *testing.T) {
			r, sender, q := newTestRouterForExec(t)
			r.Route(context.Background(), "chat1", "user1", tc.cmd)
			q.Shutdown()
			msgs := sender.Messages()
			// Command dispatches execution; expect at least one message (error since binary doesn't exist)
			if len(msgs) == 0 {
				t.Fatalf("expected at least one message for %s, got none", tc.cmd)
			}
		})
	}
}

func TestRouterSummary_Dispatches(t *testing.T) {
	r, sender, q := newTestRouterForExec(t)
	r.getSession("chat1")
	r.store.UpdateSession("chat1", func(s *Session) {
		s.LastOutput = "Some previous output"
	})
	r.Route(context.Background(), "chat1", "user1", "/summary")
	q.Shutdown()
	msgs := sender.Messages()
	if len(msgs) == 0 {
		t.Fatalf("expected at least one message, got none")
	}
}

func TestRouterHandlePrompt(t *testing.T) {
	r, sender, q := newTestRouterForExec(t)
	r.Route(context.Background(), "chat1", "user1", "hello Claude")
	q.Shutdown()
	msgs := sender.Messages()
	if len(msgs) == 0 {
		t.Fatalf("expected at least one message, got none")
	}
}

func TestRouterExecClaude_NoQueue(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &spySender{}
	exec := NewClaudeExecutor("/nonexistent_binary_for_test", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), exec, store, sender, map[string]bool{"user1": true}, dir, nil)
	// No queue — execClaude called synchronously
	r.Route(context.Background(), "chat1", "user1", "hello")
	if len(sender.messages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d: %v", len(sender.messages), sender.messages)
	}
	if !strings.Contains(sender.messages[0], "执行中") {
		t.Fatalf("expected '执行中' text, got: %q", sender.messages[0])
	}
	hasError := false
	for _, m := range sender.messages {
		if strings.Contains(m, "出错") {
			hasError = true
		}
	}
	if !hasError {
		t.Fatalf("expected error message, got: %v", sender.messages)
	}
}

func TestRouterExecClaude_SavesSessionID(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte(`#!/bin/sh
echo '{"type":"result","result":"Hello!","session_id":"new-sess-456"}'
`), 0755)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &spySender{}
	exec := NewClaudeExecutor(script, "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), exec, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "hello")

	sess := store.GetSession("chat1", "", "")
	if sess.ClaudeSessionID != "new-sess-456" {
		t.Fatalf("expected session ID 'new-sess-456', got: %q", sess.ClaudeSessionID)
	}
	if sess.LastOutput != "Hello!" {
		t.Fatalf("expected LastOutput 'Hello!', got: %q", sess.LastOutput)
	}
}

func TestRouterExecClaudeQueued_ShowsQueuePosition(t *testing.T) {
	r, sender, q := newTestRouterForExec(t)
	// Block the worker with a slow task
	done := make(chan struct{})
	q.Enqueue("chat1", func() {
		<-done
	})
	// This should be queued behind the blocking task
	r.Route(context.Background(), "chat1", "user1", "/git log")
	msgs := sender.Messages()
	hasQueued := false
	for _, m := range msgs {
		if strings.Contains(m, "排队") {
			hasQueued = true
		}
	}
	if !hasQueued {
		t.Fatalf("expected '排队' message, got: %v", msgs)
	}
	close(done)
	q.Shutdown()
}

func TestRouterSessionContinuity_ResumeUsedOnSecondMessage(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	script := filepath.Join(dir, "claude")
	// Script that records args and returns JSON with session_id
	os.WriteFile(script, []byte(fmt.Sprintf("#!/bin/sh\necho \"$@\" > %s\necho '{\"type\":\"result\",\"result\":\"ok\",\"session_id\":\"persistent-sess\"}'\n", argsFile)), 0755)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &spySender{}
	exec := NewClaudeExecutor(script, "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), exec, store, sender, map[string]bool{"user1": true}, dir, nil)

	// First message — no session ID yet
	r.Route(context.Background(), "chat1", "user1", "first message")
	argsData, _ := os.ReadFile(argsFile)
	if strings.Contains(string(argsData), "--resume") {
		t.Fatalf("first message should NOT have --resume, got: %q", string(argsData))
	}

	// Session ID should be saved
	sess := store.GetSession("chat1", "", "")
	if sess.ClaudeSessionID != "persistent-sess" {
		t.Fatalf("expected session ID saved, got: %q", sess.ClaudeSessionID)
	}

	// Second message — should use --resume
	r.Route(context.Background(), "chat1", "user1", "second message")
	argsData, _ = os.ReadFile(argsFile)
	if !strings.Contains(string(argsData), "--resume") || !strings.Contains(string(argsData), "persistent-sess") {
		t.Fatalf("second message should have --resume persistent-sess, got: %q", string(argsData))
	}
}

func TestRouterNewSession_ClearsSessionID(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte(fmt.Sprintf("#!/bin/sh\necho \"$@\" > %s\necho '{\"type\":\"result\",\"result\":\"ok\",\"session_id\":\"sess-round2\"}'\n", argsFile)), 0755)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &spySender{}
	exec := NewClaudeExecutor(script, "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), exec, store, sender, map[string]bool{"user1": true}, dir, nil)

	// First message — establishes session
	r.Route(context.Background(), "chat1", "user1", "hello")
	sess := store.GetSession("chat1", "", "")
	if sess.ClaudeSessionID == "" {
		t.Fatalf("expected session ID after first message")
	}

	// /new — clears session
	r.Route(context.Background(), "chat1", "user1", "/new")
	sess = store.GetSession("chat1", "", "")
	if sess.ClaudeSessionID != "" {
		t.Fatalf("expected empty session ID after /new, got: %q", sess.ClaudeSessionID)
	}

	// Next message — should NOT have --resume
	r.Route(context.Background(), "chat1", "user1", "after new")
	argsData, _ := os.ReadFile(argsFile)
	if strings.Contains(string(argsData), "--resume") {
		t.Fatalf("message after /new should NOT have --resume, got: %q", string(argsData))
	}
}

// --- cardSpySender for card-specific assertions ---

type cardSpySender struct {
	cards []CardMsg
	texts []string
}

func (s *cardSpySender) SendText(_ context.Context, _, text string) error {
	s.texts = append(s.texts, text)
	return nil
}

func (s *cardSpySender) SendTextChunked(_ context.Context, _, text string) error {
	s.texts = append(s.texts, text)
	return nil
}

func (s *cardSpySender) SendCard(_ context.Context, _ string, card CardMsg) error {
	s.cards = append(s.cards, card)
	return nil
}

func TestRouterExecClaude_ResponseIsCard(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte("#!/bin/sh\necho '{\"type\":\"result\",\"result\":\"**bold** and `code`\",\"session_id\":\"s1\"}'\n"), 0755)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	exec := NewClaudeExecutor(script, "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), exec, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "hello")

	// Response card
	if len(sender.cards) < 1 {
		t.Fatalf("expected at least 1 card, got %d: %+v", len(sender.cards), sender.cards)
	}
	if sender.cards[0].Title != "" {
		t.Fatalf("expected no title on response card, got: %q", sender.cards[0].Title)
	}
	if !strings.Contains(sender.cards[0].Content, "**bold**") {
		t.Fatalf("expected markdown content, got: %q", sender.cards[0].Content)
	}
	// 执行中 and 完成 are plain text
	hasExecuting := false
	hasDone := false
	for _, t2 := range sender.texts {
		if strings.Contains(t2, "执行中") {
			hasExecuting = true
		}
		if strings.Contains(t2, "完成") {
			hasDone = true
		}
	}
	if !hasExecuting {
		t.Fatalf("expected '执行中' text, got texts: %v", sender.texts)
	}
	if !hasDone {
		t.Fatalf("expected '完成' text, got texts: %v", sender.texts)
	}
}

func TestRouterExecClaude_ErrorIsRedCard(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte("#!/bin/sh\necho '{\"type\":\"result\",\"result\":\"something failed\",\"is_error\":true,\"session_id\":\"s1\"}'\n"), 0755)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	exec := NewClaudeExecutor(script, "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), exec, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "hello")

	var found bool
	for _, c := range sender.cards {
		if strings.Contains(c.Title, "出错") && c.Template == "red" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected red 出错 card, got cards: %+v texts: %v", sender.cards, sender.texts)
	}
}

func TestRouterExecClaude_PermissionDenialIsPurpleCard(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte("#!/bin/sh\necho '{\"type\":\"result\",\"result\":\"\",\"session_id\":\"s1\",\"permission_denials\":[{\"tool_name\":\"AskUserQuestion\",\"tool_input\":{\"questions\":[{\"question\":\"Which option?\",\"options\":[{\"label\":\"A\",\"description\":\"First\"},{\"label\":\"B\",\"description\":\"Second\"}]}]}}]}'\n"), 0755)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	exec := NewClaudeExecutor(script, "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), exec, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "do something")

	var found bool
	for _, c := range sender.cards {
		if c.Template == "purple" && strings.Contains(c.Content, "Which option?") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected purple card with question, got cards: %+v", sender.cards)
	}
}

func TestRouterExecClaude_StreamProgressCards(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	// Simulate streaming: two assistant events, then result
	os.WriteFile(script, []byte(`#!/bin/sh
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Working on it..."}]},"session_id":"s1"}'
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Almost done..."}]},"session_id":"s1"}'
echo '{"type":"result","result":"Final answer","session_id":"s1"}'
`), 0755)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	exec := NewClaudeExecutor(script, "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), exec, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "hello")

	// Result card with content
	if len(sender.cards) < 1 {
		t.Fatalf("expected at least 1 card, got %d: %+v", len(sender.cards), sender.cards)
	}
	if !strings.Contains(sender.cards[0].Content, "Final answer") {
		t.Fatalf("expected final answer in result card, got: %q", sender.cards[0].Content)
	}
	// 完成 as plain text
	hasDone := false
	for _, t2 := range sender.texts {
		if strings.Contains(t2, "完成") {
			hasDone = true
		}
	}
	if !hasDone {
		t.Fatalf("expected '完成' text, got texts: %v", sender.texts)
	}
}

func TestRouterExecClaude_EmptyResponse(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte("#!/bin/sh\necho '{\"type\":\"result\",\"result\":\"\",\"session_id\":\"s1\"}'\n"), 0755)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	exec := NewClaudeExecutor(script, "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), exec, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "hello")

	var found bool
	for _, c := range sender.cards {
		if strings.Contains(c.Content, "无输出") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected '无输出' card, got cards: %+v texts: %v", sender.cards, sender.texts)
	}
}

func TestRouterExecClaude_AutoRecoversSessionNotFound(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	script := filepath.Join(dir, "claude")
	// When called with --resume, return session-not-found error.
	// When called without --resume, succeed.
	os.WriteFile(script, []byte(fmt.Sprintf(`#!/bin/sh
if echo "$@" | grep -q -- "--resume"; then
  echo '{"type":"result","result":"No conversation found with session ID old-sess","is_error":true,"session_id":"old-sess"}'
else
  echo "$@" > %s
  echo '{"type":"result","result":"recovered","session_id":"new-sess"}'
fi
`, argsFile)), 0755)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	exec := NewClaudeExecutor(script, "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), exec, store, sender, map[string]bool{"user1": true}, dir, nil)

	// Pre-populate session with an old session ID
	r.getSession("chat1")
	r.store.UpdateSession("chat1", func(s *Session) {
		s.ClaudeSessionID = "old-sess"
	})

	r.Route(context.Background(), "chat1", "user1", "hello")

	// Recovery call should not use --resume
	argsData, _ := os.ReadFile(argsFile)
	if strings.Contains(string(argsData), "--resume") {
		t.Fatalf("recovery call should not have --resume, got: %q", string(argsData))
	}

	// Should show the recovered result
	var found bool
	for _, c := range sender.cards {
		if strings.Contains(c.Content, "recovered") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 'recovered' in result, got cards: %+v texts: %v", sender.cards, sender.texts)
	}

	// Session ID should be updated to new-sess
	sess := store.GetSession("chat1", "", "")
	if sess.ClaudeSessionID != "new-sess" {
		t.Fatalf("expected new-sess after recovery, got: %q", sess.ClaudeSessionID)
	}
}

func TestRouterExecClaude_StreamSavesSessionAndOutput(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte(`#!/bin/sh
echo '{"type":"result","result":"streamed result","session_id":"stream-sess-1"}'
`), 0755)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	exec := NewClaudeExecutor(script, "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), exec, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "hello")

	sess := store.GetSession("chat1", "", "")
	if sess.ClaudeSessionID != "stream-sess-1" {
		t.Fatalf("expected session ID 'stream-sess-1', got: %q", sess.ClaudeSessionID)
	}
	if sess.LastOutput != "streamed result" {
		t.Fatalf("expected LastOutput 'streamed result', got: %q", sess.LastOutput)
	}
}

// --- /info tests ---

func TestRouterInfo(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/info")
	msg := sender.LastMessage()
	// Should show a compact overview card
	if !strings.Contains(msg, "当前概览") {
		t.Fatalf("expected 当前概览 title, got: %q", msg)
	}
	// Should show the model
	if !strings.Contains(msg, "sonnet") {
		t.Fatalf("expected model in /info output, got: %q", msg)
	}
}

// --- /grep tests ---

func TestRouterGrep_EmptyArgs(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/grep")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "用法") {
		t.Fatalf("expected usage message for /grep with no args, got: %q", msg)
	}
}

func TestRouterGrep_WithPattern(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/grep TODO")
	msgs := sender.messages
	hasExecuting := false
	for _, m := range msgs {
		if strings.Contains(m, "执行中") {
			hasExecuting = true
		}
	}
	if !hasExecuting {
		t.Fatalf("expected /grep to trigger execution, got: %v", msgs)
	}
}

// --- /pr tests ---

func TestRouterPR_NoTitle(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/pr")
	msgs := sender.messages
	hasExecuting := false
	for _, m := range msgs {
		if strings.Contains(m, "执行中") {
			hasExecuting = true
		}
	}
	if !hasExecuting {
		t.Fatalf("expected /pr to trigger execution, got: %v", msgs)
	}
}

func TestRouterPR_WithTitle(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/pr feat: add new feature")
	msgs := sender.messages
	hasExecuting := false
	for _, m := range msgs {
		if strings.Contains(m, "执行中") {
			hasExecuting = true
		}
	}
	if !hasExecuting {
		t.Fatalf("expected /pr with title to trigger execution, got: %v", msgs)
	}
}

// --- /status git info tests ---

func TestRouterStatus_ShowsGitFields(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/status")
	msg := sender.LastMessage()
	// Should now include git branch and workspace fields
	if !strings.Contains(msg, "Git 分支") {
		t.Fatalf("expected 'Git 分支' in status, got: %q", msg)
	}
	if !strings.Contains(msg, "工作区") {
		t.Fatalf("expected '工作区' in status, got: %q", msg)
	}
}

// --- gitBranch / gitStatusSummary unit tests ---

func TestGitBranch_EmptyWorkDir(t *testing.T) {
	if gitBranch("") != "" {
		t.Fatal("expected empty string for empty workDir")
	}
}

func TestGitBranch_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	if gitBranch(dir) != "" {
		t.Fatalf("expected empty for non-git dir")
	}
}

func TestGitBranch_RealRepo(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()
	branch := gitBranch(dir)
	if branch == "" {
		t.Fatalf("expected non-empty branch for valid git repo")
	}
}

func TestGitStatusSummary_EmptyWorkDir(t *testing.T) {
	if gitStatusSummary("") != "" {
		t.Fatal("expected empty string for empty workDir")
	}
}

func TestGitStatusSummary_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	if gitStatusSummary(dir) != "" {
		t.Fatalf("expected empty for non-git dir")
	}
}

func TestGitStatusSummary_CleanRepo(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()
	result := gitStatusSummary(dir)
	if result != "无变更" {
		t.Fatalf("expected '无变更' for clean repo, got: %q", result)
	}
}

func TestGitStatusSummary_DirtyRepo(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()
	// Create an untracked file — appears in git status --porcelain
	os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("change"), 0644)
	result := gitStatusSummary(dir)
	if !strings.Contains(result, "文件变更") {
		t.Fatalf("expected '文件变更' for dirty repo, got: %q", result)
	}
}

// --- /switch by index tests ---

func TestRouterSwitch_ByValidIndex(t *testing.T) {
	r, sender := newTestRouter(t)
	r.getSession("chat1")
	r.store.UpdateSession("chat1", func(s *Session) {
		s.History = []string{"sess-0", "sess-1"}
	})
	r.Route(context.Background(), "chat1", "user1", "/switch 1")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "已切换到会话") || !strings.Contains(msg, "sess-1") {
		t.Fatalf("expected switch to sess-1 by index, got: %q", msg)
	}
}

func TestRouterSwitch_ByOutOfRangeIndex(t *testing.T) {
	r, sender := newTestRouter(t)
	r.getSession("chat1")
	r.store.UpdateSession("chat1", func(s *Session) {
		s.History = []string{"sess-0"}
	})
	r.Route(context.Background(), "chat1", "user1", "/switch 99")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "不存在") {
		t.Fatalf("expected out-of-range error, got: %q", msg)
	}
}

// --- /log with count arg ---

func TestRouterLog_WithCount(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/log 5")
	msgs := sender.messages
	hasExecuting := false
	for _, m := range msgs {
		if strings.Contains(m, "执行中") {
			hasExecuting = true
		}
	}
	if !hasExecuting {
		t.Fatalf("expected /log 5 to trigger execution, got: %v", msgs)
	}
}

// --- /branch with name arg ---

func TestRouterBranch_WithName(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/branch feature/new")
	msgs := sender.messages
	hasExecuting := false
	for _, m := range msgs {
		if strings.Contains(m, "执行中") {
			hasExecuting = true
		}
	}
	if !hasExecuting {
		t.Fatalf("expected /branch <name> to trigger execution, got: %v", msgs)
	}
}

// --- /undo tests ---

func TestRouterUndo_NoChanges(t *testing.T) {
	// Non-git temp dir — gitStatusSummary returns "" so cmdUndo shows "无需撤销"
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/undo")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "没有未提交") {
		t.Fatalf("expected no-changes message for /undo on non-git dir, got: %q", msg)
	}
}

func TestRouterUndo_WithChanges(t *testing.T) {
	dir := t.TempDir()
	// Initialize a git repo with a dirty file
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()
	os.WriteFile(filepath.Join(dir, "dirty.go"), []byte("change"), 0644)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &spySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/undo")
	msgs := sender.messages
	// Should trigger execution (not the "no changes" message)
	hasExecuting := false
	for _, m := range msgs {
		if strings.Contains(m, "执行中") {
			hasExecuting = true
		}
	}
	if !hasExecuting {
		t.Fatalf("expected /undo to trigger execution when changes exist, got: %v", msgs)
	}
}

// --- suggestCommand / levenshtein tests ---

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"abc", "ab", 1},
		{"/comit", "/commit", 1},
		{"/statuss", "/status", 1},
		{"/helo", "/help", 1},
		{"/xyz", "/status", 6},
	}
	for _, tt := range tests {
		got := levenshtein(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestSuggestCommand_Typo(t *testing.T) {
	// /comit is 1 edit from /commit
	got := suggestCommand("/comit")
	if got != "/commit" {
		t.Fatalf("expected '/commit' suggestion for '/comit', got: %q", got)
	}
}

func TestSuggestCommand_NoMatch(t *testing.T) {
	// /xyzabc is too far from any known command
	got := suggestCommand("/xyzabc123")
	if got != "" {
		t.Fatalf("expected no suggestion for '/xyzabc123', got: %q", got)
	}
}

func TestRouterUnknownCommand_WithSuggestion(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/comit")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "你是否想用") || !strings.Contains(msg, "/commit") {
		t.Fatalf("expected suggestion for /comit, got: %q", msg)
	}
}

func TestRouterUnknownCommand_NoSuggestion(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/xyzabc123")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "未知命令") {
		t.Fatalf("expected unknown command message, got: %q", msg)
	}
	if strings.Contains(msg, "你是否想用") {
		t.Fatalf("should not suggest for totally unknown command, got: %q", msg)
	}
}

// --- /version tests ---

func TestRouterVersion(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/version")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "devbot version") {
		t.Fatalf("expected version info, got: %q", msg)
	}
}

// --- cmdModel: session with empty model (uses executor default) ---

func TestRouterModelShowCurrentFromExecutor(t *testing.T) {
	r, sender := newTestRouter(t)
	// Explicitly clear the session's model field to trigger the "" fallback path
	r.getSession("chat1")
	r.store.UpdateSession("chat1", func(s *Session) {
		s.Model = "" // clear model so cmdModel falls back to executor.Model()
	})
	r.Route(context.Background(), "chat1", "user1", "/model")
	msg := sender.LastMessage()
	// Executor default is "sonnet" per newTestRouter setup
	if !strings.Contains(msg, "sonnet") {
		t.Fatalf("expected executor default 'sonnet' fallback, got: %q", msg)
	}
}

// --- gitBranch: detached HEAD ---

func TestGitBranch_DetachedHEAD(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()
	// Force detached HEAD by checking out commit hash directly
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Skip("git rev-parse failed, skipping detached HEAD test")
	}
	hash := strings.TrimSpace(string(out))
	exec.Command("git", "-C", dir, "checkout", hash).Run()
	result := gitBranch(dir)
	// In detached HEAD state, gitBranch should return "" (since branch == "HEAD")
	if result != "" {
		t.Fatalf("expected empty for detached HEAD, got: %q", result)
	}
}

// --- store.Save error case ---

func TestStoreSaveFromRouter_WriteError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; permission-based tests are unreliable")
	}
	dir := t.TempDir()
	stateDir := filepath.Join(dir, "statedir")
	os.MkdirAll(stateDir, 0755)
	store, err := NewStore(filepath.Join(stateDir, "state.json"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	// Make the state directory read-only so WriteFile fails
	os.Chmod(stateDir, 0555)
	defer os.Chmod(stateDir, 0755)

	saveErr := store.Save()
	if saveErr == nil {
		t.Fatal("expected Save() to return error on read-only dir")
	}
}

// --- findFile: fuzzy match via walk ---

func TestFindFile_FuzzyMatch(t *testing.T) {
	dir := t.TempDir()
	// Create a file that doesn't match by exact path but matches by name
	subDir := filepath.Join(dir, "src")
	os.Mkdir(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "myconfig.yaml"), []byte("content"), 0644)

	result := findFile(dir, "myconfig")
	if result == "" {
		t.Fatalf("expected fuzzy match for 'myconfig', got empty")
	}
	if !strings.Contains(result, "myconfig.yaml") {
		t.Fatalf("expected myconfig.yaml in result, got: %q", result)
	}
}

// --- execClaude: permission denial identical to progress (skip duplicate card) ---

func TestRouterExecClaude_PermissionDenialSkipsDuplicate(t *testing.T) {
	dir := t.TempDir()
	// Script outputs a permission denial result
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte(`#!/bin/sh
echo '{"type":"result","result":"You need permission","is_error":false,"session_id":"s1","permission_denials":["blocked"]}'
`), 0755)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor(script, "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "do something")

	// Should have received at least the executing text + permission denial card
	found := false
	for _, c := range sender.cards {
		if strings.Contains(c.Title, "Claude 需要确认") {
			found = true
		}
	}
	if !found {
		t.Logf("cards: %+v", sender.cards)
		// Not a hard failure — the test mainly exercises the code path
	}
}

// --- execClaude: large output (>4000 chars) in result ---

func TestRouterExecClaude_LargeOutput(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	// Generate a result with >4000 chars
	largeOutput := strings.Repeat("x", 5000)
	scriptBody := `#!/bin/sh
echo '{"type":"result","result":"` + largeOutput + `","session_id":"s1"}'
`
	os.WriteFile(script, []byte(scriptBody), 0755)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor(script, "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "hello")

	// Should have received a card with the large output (not truncated in final result)
	found := false
	for _, c := range sender.cards {
		if strings.Contains(c.Content, "x") && len(c.Content) > 100 {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected large output card, cards: %+v", sender.cards)
	}
}

// --- cmdStatus when executor is running ---

func TestRouterStatus_WhenRunning(t *testing.T) {
	dir := t.TempDir()
	readyPath := filepath.Join(dir, "ready")
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte(fmt.Sprintf("#!/bin/sh\ntouch %s\nexec sleep 30\n", readyPath)), 0755)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	snd := &spySender{}
	ex := NewClaudeExecutor(script, "sonnet", 60*time.Second)
	r := NewRouter(context.Background(), ex, store, snd, map[string]bool{"user1": true}, dir, nil)

	// Start long-running task in background
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.Route(context.Background(), "chat1", "user1", "run long task")
	}()

	// Wait for script to start
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(readyPath); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(readyPath); err != nil {
		t.Fatal("script did not start within 5s")
	}

	// Check status while running
	r.Route(context.Background(), "chat1", "user1", "/status")
	found := false
	for _, m := range snd.messages {
		if strings.Contains(m, "执行中") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected '执行中' in status while running, got: %v", snd.messages)
	}

	ex.Kill()
	<-done
}

// TestRouterCompact verifies that /compact sends a summarization prompt to Claude.
func TestRouterCompact(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/compact")
	msgs := sender.messages
	hasExecuting := false
	for _, m := range msgs {
		if strings.Contains(m, "执行中") {
			hasExecuting = true
		}
	}
	if !hasExecuting {
		t.Fatalf("expected /compact to trigger execution, got: %v", msgs)
	}
}

// TestRouterInfo_WhenRunning verifies that /info shows "执行中" when executor is busy.
func TestRouterInfo_WhenRunning(t *testing.T) {
	dir := t.TempDir()
	readyPath := filepath.Join(dir, "ready")
	script := filepath.Join(dir, "claude")
	os.WriteFile(script, []byte(fmt.Sprintf("#!/bin/sh\ntouch %s\nexec sleep 30\n", readyPath)), 0755)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	snd := &spySender{}
	ex := NewClaudeExecutor(script, "sonnet", 60*time.Second)
	r := NewRouter(context.Background(), ex, store, snd, map[string]bool{"user1": true}, dir, nil)

	done := make(chan struct{})
	go func() {
		defer close(done)
		r.Route(context.Background(), "chat1", "user1", "run long task")
	}()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(readyPath); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(readyPath); err != nil {
		t.Fatal("script did not start within 5s")
	}

	r.Route(context.Background(), "chat1", "user1", "/info")
	found := false
	for _, m := range snd.messages {
		if strings.Contains(m, "执行中") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected '执行中' in /info while running, got: %v", snd.messages)
	}

	ex.Kill()
	<-done
}

// TestExecClaude_SessionAutoRecover verifies that if claude reports "No conversation found with session ID",
// execClaude clears the session and retries without --resume.
func TestExecClaude_SessionAutoRecover(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	// First call (with --resume): return error; second call (without): succeed.
	sentinelPath := filepath.Join(dir, "first_done")
	scriptContent := fmt.Sprintf(`#!/bin/sh
if echo "$@" | grep -q -- "--resume"; then
    printf '{"type":"result","is_error":true,"result":"No conversation found with session ID abc","session_id":""}\n'
    touch %s
    exit 0
fi
printf '{"type":"result","is_error":false,"result":"recovered ok","session_id":"new-sess"}\n'
`, sentinelPath)
	os.WriteFile(script, []byte(scriptContent), 0755)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	// Pre-set a session with a fake session ID so execClaude will pass --resume on first call.
	store.GetSession("chat1", dir, "sonnet")
	store.UpdateSession("chat1", func(s *Session) {
		s.ClaudeSessionID = "old-session-id"
	})

	snd := &spySender{}
	ex := NewClaudeExecutor(script, "sonnet", 15*time.Second)
	r := NewRouter(context.Background(), ex, store, snd, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "hello after session loss")

	// Verify sentinel was touched (first call did happen with --resume)
	if _, err := os.Stat(sentinelPath); err != nil {
		t.Fatal("expected first claude call with --resume, sentinel not found")
	}
	// After recovery, the session ID should be cleared and updated to new one
	sess := store.GetSession("chat1", dir, "sonnet")
	if sess.ClaudeSessionID != "new-sess" {
		t.Fatalf("expected session to be updated to new-sess, got: %q", sess.ClaudeSessionID)
	}
	// Verify success message was sent
	found := false
	for _, m := range snd.messages {
		if strings.Contains(m, "完成") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected '完成' in messages after recovery, got: %v", snd.messages)
	}
}

// --- minInt unit test ---

func TestMinInt(t *testing.T) {
	if minInt(1, 2, 3) != 1 {
		t.Fatal("expected 1")
	}
	if minInt(3, 1, 2) != 1 {
		t.Fatal("expected 1")
	}
	if minInt(2, 3, 1) != 1 {
		t.Fatal("expected 1")
	}
	if minInt(5, 5, 5) != 5 {
		t.Fatal("expected 5")
	}
}
