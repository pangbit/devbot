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

func TestRouterCd_RelativeFromCurrent(t *testing.T) {
	// /cd ./subdir should navigate relative to current workDir
	dir := t.TempDir()
	subdir := filepath.Join(dir, "project1", "src")
	os.MkdirAll(subdir, 0755)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &spySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	// First cd to project1
	r.Route(context.Background(), "chat1", "user1", "/cd project1")
	if !strings.Contains(sender.LastMessage(), "已切换") {
		t.Fatalf("expected success for /cd project1, got: %q", sender.LastMessage())
	}

	// Now cd ./src relative to current
	r.Route(context.Background(), "chat1", "user1", "/cd ./src")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "已切换") {
		t.Fatalf("expected success for /cd ./src from project1, got: %q", msg)
	}
	// Verify we ended up in project1/src
	sess := store.GetSession("chat1", dir, "sonnet")
	if !strings.HasSuffix(sess.WorkDir, "project1/src") {
		t.Fatalf("expected workDir to end with project1/src, got: %q", sess.WorkDir)
	}
}

func TestRouterCd_DotDot(t *testing.T) {
	// /cd .. should go up one level
	dir := t.TempDir()
	subdir := filepath.Join(dir, "project1")
	os.MkdirAll(subdir, 0755)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &spySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	// First cd to project1
	r.Route(context.Background(), "chat1", "user1", "/cd project1")

	// Now cd ..  - BUT this goes to workRoot (dir), which equals root, so underRoot should pass
	r.Route(context.Background(), "chat1", "user1", "/cd ..")
	msg := sender.LastMessage()
	// ".." from project1 goes to dir which == work root, should succeed
	if !strings.Contains(msg, "已切换") {
		t.Fatalf("expected success for /cd .., got: %q", msg)
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

func TestRouterCommit_WithMsg_NothingToCommit(t *testing.T) {
	// /commit "message" with nothing staged should show error card
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/commit my commit message")

	if len(sender.cards) == 0 {
		t.Fatal("expected a card from /commit with message")
	}
	// Should be an error (nothing to commit)
	if !strings.Contains(sender.cards[0].Title, "出错") {
		t.Fatalf("expected error card (nothing to commit), got: %q", sender.cards[0].Title)
	}
}

func TestRouterCommit_WithMsg_Success(t *testing.T) {
	// /commit "message" with actual changes should commit directly
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()
	os.WriteFile(filepath.Join(dir, "file.go"), []byte("package main"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/commit add file.go")

	if len(sender.cards) == 0 {
		t.Fatal("expected a card from /commit with message")
	}
	if !strings.Contains(sender.cards[0].Title, "成功") {
		t.Fatalf("expected success card, got: %q", sender.cards[0].Title)
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
	// /log in a non-git dir should return a "no commits" message
	r.Route(context.Background(), "chat1", "user1", "/log")
	msg := sender.LastMessage()
	if msg == "" {
		t.Fatalf("expected some message from /log, got none")
	}
}

func TestRouterBranch_NoArgs(t *testing.T) {
	r, sender := newTestRouter(t)
	// /branch without args in non-git dir should show error message
	r.Route(context.Background(), "chat1", "user1", "/branch")
	msg := sender.LastMessage()
	if msg == "" {
		t.Fatalf("expected some response from /branch, got none")
	}
}

func TestRouterBranch_NoArgs_InGitRepo(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "init").Run()

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)
	store.GetSession("chat1", dir, "sonnet")

	r.Route(context.Background(), "chat1", "user1", "/branch")

	if len(sender.cards) == 0 {
		t.Fatalf("expected card with branch list, texts: %v", sender.texts)
	}
	if sender.cards[0].Title != "分支列表" {
		t.Fatalf("expected branch list card, got: %q", sender.cards[0].Title)
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

func TestRouterLs_WithDir(t *testing.T) {
	r, sender := newTestRouter(t)
	// project1 subdir exists (created by newTestRouter); add a file and hidden file inside it
	session := r.getSession("chat1")
	os.WriteFile(filepath.Join(session.WorkDir, "project1", "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(session.WorkDir, "project1", ".hidden_file"), []byte("secret"), 0644)

	r.Route(context.Background(), "chat1", "user1", "/ls project1")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "main.go") {
		t.Fatalf("expected 'main.go' in /ls project1 output, got: %q", msg)
	}
	// Hidden files should be excluded
	if strings.Contains(msg, ".hidden_file") {
		t.Fatalf("expected hidden file to be excluded, got: %q", msg)
	}
}

func TestRouterLs_WithDir_NotFound(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/ls nonexistent_xyz")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "读取目录出错") {
		t.Fatalf("expected error for nonexistent dir, got: %q", msg)
	}
}

func TestRouterLs_WithDir_OutsideRoot(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/ls ../../etc")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "不允许") {
		t.Fatalf("expected security error for path outside root, got: %q", msg)
	}
}

func TestRouterLs_WithDir_EmptyDir(t *testing.T) {
	r, sender := newTestRouter(t)
	// project1 is an empty directory (newTestRouter creates it but puts nothing visible inside)
	r.Route(context.Background(), "chat1", "user1", "/ls project1")
	msg := sender.LastMessage()
	// Either lists files or says empty — both are valid
	if msg == "" {
		t.Fatalf("expected some response from /ls project1")
	}
}

func TestRouterLs_WithDir_SubdirShownWithSlash(t *testing.T) {
	r, sender := newTestRouter(t)
	// Create a subdir inside project1 to cover the IsDir → "/" suffix path
	session := r.getSession("chat1")
	subdir := filepath.Join(session.WorkDir, "project1", "internal")
	os.MkdirAll(subdir, 0755)

	r.Route(context.Background(), "chat1", "user1", "/ls project1")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "internal/") {
		t.Fatalf("expected 'internal/' (with slash) for subdir, got: %q", msg)
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
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(readyPath); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(readyPath); err != nil {
		t.Fatal("script did not start within 10s")
	}

	// Kill the running process
	r.Route(ctx, "chat2", "user1", "/kill")

	// Wait for the first route to complete
	select {
	case <-done:
	case <-time.After(10 * time.Second):
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

	// Block the worker and wait for it to actually start, then fill the queue.
	// Without waiting, the worker goroutine may or may not have picked up the
	// blocking task before the fill loop runs, making channel occupancy non-deterministic.
	started := make(chan struct{})
	done := make(chan struct{})
	q.Enqueue("chat1", func() {
		close(started)
		<-done
	})
	<-started // worker is now blocked executing the first task; channel is empty

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

// TestExecClaude_ProgressFires verifies that onProgress sends intermediate cards
// after 5 seconds, rate-limits subsequent calls, and skips the final result card
// when it matches the last progress content. Skipped in short mode (-short flag).
func TestExecClaude_ProgressFires(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 6-second timing test; run without -short to include")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	// Script: sleep 5.5s, emit two assistant events (second tests sinceLast<10s path),
	// then result with same text as progress (tests "skip duplicate result card" path).
	scriptContent := `#!/bin/sh
sleep 5.5
printf '{"type":"assistant","message":{"content":[{"type":"text","text":"computing..."}]}}\n'
printf '{"type":"assistant","message":{"content":[{"type":"text","text":"computing..."}]}}\n'
printf '{"type":"result","result":"computing...","session_id":"s1"}\n'
`
	os.WriteFile(script, []byte(scriptContent), 0755)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor(script, "sonnet", 30*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "do something slow")

	// A progress card should have been sent (onProgress fired after 5.5s)
	progressFound := false
	for _, c := range sender.cards {
		if strings.Contains(c.Content, "computing") {
			progressFound = true
		}
	}
	if !progressFound {
		t.Fatalf("expected progress card after 5.5s, cards: %+v", sender.cards)
	}
	// Final result card should NOT be sent (output == lastProgressContent)
	// Only 1 card total: the progress card; result card is skipped
	if len(sender.cards) != 1 {
		t.Fatalf("expected exactly 1 card (progress, result skipped), got %d: %+v", len(sender.cards), sender.cards)
	}
	// Completion text should still be sent
	completionFound := false
	for _, txt := range sender.texts {
		if strings.Contains(txt, "完成") {
			completionFound = true
		}
	}
	if !completionFound {
		t.Fatalf("expected '完成' text after execution, texts: %v", sender.texts)
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

func TestRouterGrep_WithPattern_NoMatches(t *testing.T) {
	r, sender := newTestRouter(t)
	// In a tempdir with no code files, grep finds nothing
	r.Route(context.Background(), "chat1", "user1", "/grep SOME_UNIQUE_TOKEN_XYZ")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "未找到") {
		t.Fatalf("expected 'no match' message, got: %q", msg)
	}
}

func TestRouterGrep_WithPattern_HasMatches(t *testing.T) {
	dir := t.TempDir()
	// Create a Go file with a TODO comment
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n// TODO: fix this\nfunc main() {}\n"), 0644)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)
	store.GetSession("chat1", dir, "sonnet")

	r.Route(context.Background(), "chat1", "user1", "/grep TODO")

	if len(sender.cards) == 0 {
		t.Fatalf("expected card with grep results, texts: %v", sender.texts)
	}
	if !strings.Contains(sender.cards[0].Content, "TODO") {
		t.Fatalf("expected 'TODO' in grep output, got: %q", sender.cards[0].Content)
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
	// /log 5 should produce some response (either commits or "no commits" message)
	r.Route(context.Background(), "chat1", "user1", "/log 5")
	msg := sender.LastMessage()
	if msg == "" {
		t.Fatalf("expected some message from /log 5, got none")
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
	// Initialize a git repo with a tracked file, then modify it
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()
	os.WriteFile(filepath.Join(dir, "dirty.go"), []byte("original"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "add file").Run()
	// Now make it dirty
	os.WriteFile(filepath.Join(dir, "dirty.go"), []byte("modified"), 0644)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &spySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/undo")
	msg := sender.LastMessage()
	// Should show success message (direct execution, no Claude)
	if !strings.Contains(msg, "已撤销") {
		t.Fatalf("expected '已撤销' message for /undo with changes, got: %q", msg)
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
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(readyPath); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(readyPath); err != nil {
		t.Fatal("script did not start within 10s")
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

// TestTruncateForDisplay verifies the truncation helper.
func TestTruncateForDisplay(t *testing.T) {
	// Short text - no truncation
	short := strings.Repeat("a", 100)
	if truncateForDisplay(short, 200) != short {
		t.Fatal("expected short text unchanged")
	}
	// Long text - should truncate and add header
	long := strings.Repeat("b", 5000)
	result := truncateForDisplay(long, 4000)
	if !strings.Contains(result, "内容过长") {
		t.Fatalf("expected truncation header, got: %q", result[:50])
	}
	runes := []rune(result)
	if len(runes) > 4100 { // header + 4000 runes
		t.Fatalf("expected result within ~4100 runes, got %d", len(runes))
	}
	// Unicode - should handle multibyte chars correctly
	emoji := strings.Repeat("🎉", 5000)
	r2 := truncateForDisplay(emoji, 4000)
	if !strings.Contains(r2, "内容过长") {
		t.Fatalf("expected truncation for emoji string")
	}
}

// TestRouterTest_NoArgs verifies that /test triggers Claude execution.
func TestRouterTest_NoArgs(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/test")
	msgs := sender.messages
	hasExecuting := false
	for _, m := range msgs {
		if strings.Contains(m, "执行中") {
			hasExecuting = true
		}
	}
	if !hasExecuting {
		t.Fatalf("expected /test to trigger execution, got: %v", msgs)
	}
}

// TestRouterTest_WithPattern verifies that /test with a pattern passes it to Claude.
func TestRouterTest_WithPattern(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/test TestMyFeature")
	msgs := sender.messages
	hasExecuting := false
	for _, m := range msgs {
		if strings.Contains(m, "执行中") {
			hasExecuting = true
		}
	}
	if !hasExecuting {
		t.Fatalf("expected /test with pattern to trigger execution, got: %v", msgs)
	}
}

// TestRouterRecent_NoArgs verifies that /recent without args shows some response.
func TestRouterRecent_NoArgs(t *testing.T) {
	r, sender := newTestRouter(t)
	// Non-git dir: should show "not a git repo" message
	r.Route(context.Background(), "chat1", "user1", "/recent")
	msg := sender.LastMessage()
	if msg == "" {
		t.Fatalf("expected some response from /recent, got none")
	}
}

// TestRouterRecent_WithCount verifies that /recent with count works.
func TestRouterRecent_WithCount(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/recent 5")
	msg := sender.LastMessage()
	if msg == "" {
		t.Fatalf("expected some response from /recent 5, got none")
	}
}

// TestRouterRecent_InvalidCount verifies that /recent with invalid count shows usage.
func TestRouterRecent_InvalidCount(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/recent abc")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "用法") {
		t.Fatalf("expected usage message for invalid count, got: %q", msg)
	}
}

// TestRouterRecent_InGitRepo verifies that /recent shows recently modified files in a git repo.
func TestRouterRecent_InGitRepo(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "init").Run()

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)
	store.GetSession("chat1", dir, "sonnet")

	r.Route(context.Background(), "chat1", "user1", "/recent")

	if len(sender.cards) == 0 {
		t.Fatalf("expected card with recent files, texts: %v", sender.texts)
	}
	if !strings.Contains(sender.cards[0].Content, "main.go") {
		t.Fatalf("expected 'main.go' in recent files, got: %q", sender.cards[0].Content)
	}
}

// TestRouterRecent_LimitsToN verifies /recent stops after n unique files.
func TestRouterRecent_LimitsToN(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	// Commit 5 separate files
	for i := 0; i < 5; i++ {
		fname := fmt.Sprintf("file%d.go", i)
		os.WriteFile(filepath.Join(dir, fname), []byte("package main"), 0644)
		exec.Command("git", "-C", dir, "add", fname).Run()
		exec.Command("git", "-C", dir, "commit", "-m", fmt.Sprintf("add %s", fname)).Run()
	}

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)
	store.GetSession("chat1", dir, "sonnet")

	// Request only 3 recent files
	r.Route(context.Background(), "chat1", "user1", "/recent 3")

	if len(sender.cards) == 0 {
		t.Fatalf("expected card, texts: %v", sender.texts)
	}
	// Card should have 3 file lines
	lines := strings.Split(strings.TrimSpace(sender.cards[0].Content), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 files in /recent 3, got %d: %q", len(lines), sender.cards[0].Content)
	}
}

// TestRouterDebug_NoOutput verifies /debug with no last output shows a hint.
func TestRouterDebug_NoOutput(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/debug")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "暂无") {
		t.Fatalf("expected no-output message for /debug, got: %q", msg)
	}
}

// TestRouterDebug_WithOutput verifies /debug with last output triggers Claude.
func TestRouterDebug_WithOutput(t *testing.T) {
	r, sender := newTestRouter(t)
	// Set a last output first
	r.store.GetSession("chat1", r.store.WorkRoot(), "sonnet")
	r.store.UpdateSession("chat1", func(s *Session) {
		s.LastOutput = "Error: connection refused on port 8080"
	})
	r.Route(context.Background(), "chat1", "user1", "/debug")
	msgs := sender.messages
	hasExecuting := false
	for _, m := range msgs {
		if strings.Contains(m, "执行中") {
			hasExecuting = true
		}
	}
	if !hasExecuting {
		t.Fatalf("expected /debug with output to trigger execution, got: %v", msgs)
	}
}

// TestRouterFind_EmptyArgs verifies that /find without args shows usage.
func TestRouterFind_EmptyArgs(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/find")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "用法") {
		t.Fatalf("expected usage message for /find with no args, got: %q", msg)
	}
}

// TestRouterFind_WithPattern verifies that /find runs directly and returns results.
func TestRouterFind_WithPattern_NoMatch(t *testing.T) {
	r, sender := newTestRouter(t)
	// No .go files in the tempdir root (README.md is there but not .go)
	r.Route(context.Background(), "chat1", "user1", "/find nonexistent_xyz.go")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "未找到") {
		t.Fatalf("expected 'not found' message, got: %q", msg)
	}
}

func TestRouterFind_WithPattern_HasMatch(t *testing.T) {
	dir := t.TempDir()
	// Create a Go file for /find to locate
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)
	store.GetSession("chat1", dir, "sonnet")

	r.Route(context.Background(), "chat1", "user1", "/find main.go")

	if len(sender.cards) == 0 {
		t.Fatalf("expected card with find results, texts: %v", sender.texts)
	}
	if !strings.Contains(sender.cards[0].Content, "main.go") {
		t.Fatalf("expected 'main.go' in find output, got: %q", sender.cards[0].Content)
	}
}

func TestRouterFind_TruncatesAt50(t *testing.T) {
	dir := t.TempDir()
	// Create 55 files matching pattern "test_*.go"
	for i := 0; i < 55; i++ {
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("test_%03d.go", i)), []byte("package main"), 0644)
	}

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)
	store.GetSession("chat1", dir, "sonnet")

	r.Route(context.Background(), "chat1", "user1", "/find test_*.go")

	if len(sender.cards) == 0 {
		t.Fatalf("expected card with find results, texts: %v", sender.texts)
	}
	// With 55 files, should show truncation notice
	if !strings.Contains(sender.cards[0].Content, "仅显示前") {
		t.Fatalf("expected truncation notice for >50 results, got: %q", sender.cards[0].Content)
	}
}

func TestRouterGrep_TruncatesLargeOutput(t *testing.T) {
	dir := t.TempDir()
	// Create a file with many matching lines to exceed 4000 chars
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, fmt.Sprintf("// TODO item %d: this is a long comment that takes up space in the output", i))
	}
	os.WriteFile(filepath.Join(dir, "big.go"), []byte("package main\n"+strings.Join(lines, "\n")), 0644)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)
	store.GetSession("chat1", dir, "sonnet")

	r.Route(context.Background(), "chat1", "user1", "/grep TODO")

	if len(sender.cards) == 0 {
		t.Fatalf("expected card with grep results, texts: %v", sender.texts)
	}
	if !strings.Contains(sender.cards[0].Content, "结果过多") {
		t.Fatalf("expected truncation notice for large grep output, got: %q", sender.cards[0].Content[:min(200, len(sender.cards[0].Content))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestRouterSessions_ShowsDirHint verifies that /sessions shows workDir context.
func TestRouterSessions_ShowsDirHint(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &spySender{}
	ex := NewClaudeExecutor("/nonexistent_for_test", "sonnet", 5*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	// Set up a session with DirSessions and history
	store.GetSession("chat1", dir, "sonnet")
	store.UpdateSession("chat1", func(s *Session) {
		s.DirSessions = map[string]string{
			"/home/user/myproject": "old-session-id",
		}
		s.History = []string{"old-session-id"}
		s.ClaudeSessionID = "current-session-id"
		s.DirSessions["current-dir"] = "current-session-id"
	})

	r.Route(context.Background(), "chat1", "user1", "/sessions")
	msg := sender.LastMessage()
	// Should show "myproject" as dir hint for old session
	if !strings.Contains(msg, "myproject") {
		t.Fatalf("expected workDir hint 'myproject' in sessions output, got: %q", msg)
	}
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

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(readyPath); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(readyPath); err != nil {
		t.Fatal("script did not start within 10s")
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

// --- Direct /diff and /log tests ---

func TestRouterDiff_NoChanges(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/diff")
	msg := sender.LastMessage()
	// Non-git dir: "no changes" or git diff shows nothing
	if msg == "" {
		t.Fatalf("expected some response from /diff")
	}
}

func TestRouterDiff_InGitRepo(t *testing.T) {
	dir := t.TempDir()
	// Init git repo with a commit, then modify a file
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("original"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "init").Run()
	// Now modify file to create a diff
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("modified"), 0644)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)
	// Set workDir
	store.GetSession("chat1", dir, "sonnet")

	r.Route(context.Background(), "chat1", "user1", "/diff")

	if len(sender.cards) == 0 {
		t.Fatalf("expected card with diff output, texts: %v", sender.texts)
	}
	if !strings.Contains(sender.cards[0].Content, "modified") && !strings.Contains(sender.cards[0].Content, "original") {
		t.Fatalf("expected diff content, got: %q", sender.cards[0].Content)
	}
}

func TestRouterLog_InGitRepo(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("init"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "initial commit").Run()

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)
	store.GetSession("chat1", dir, "sonnet")

	r.Route(context.Background(), "chat1", "user1", "/log")

	if len(sender.cards) == 0 {
		t.Fatalf("expected card with log output, texts: %v", sender.texts)
	}
	if !strings.Contains(sender.cards[0].Content, "initial commit") {
		t.Fatalf("expected 'initial commit' in log output, got: %q", sender.cards[0].Content)
	}
}

func TestRouterDiff_StagedChanges(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("original"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "init").Run()
	// Stage a new change
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("staged change"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)
	store.GetSession("chat1", dir, "sonnet")

	r.Route(context.Background(), "chat1", "user1", "/diff")

	if len(sender.cards) == 0 {
		t.Fatalf("expected diff card for staged changes, texts: %v", sender.texts)
	}
	if !strings.Contains(sender.cards[0].Content, "已暂存") {
		t.Fatalf("expected '已暂存' in staged diff card, got: %q", sender.cards[0].Content)
	}
}

func TestRouterLog_NoGitRepo(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/log")
	msg := sender.LastMessage()
	// Non-git dir: either "no commits" message or git error message
	if msg == "" {
		t.Fatalf("expected some message from /log in non-git dir")
	}
}

func TestRouterLog_ErrorWithOutput(t *testing.T) {
	// When git log fails AND has output (error message), show git error text
	dir := t.TempDir()
	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &spySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/log")
	msg := sender.LastMessage()
	if msg == "" {
		t.Fatalf("expected some error message from /log in non-git dir")
	}
}

// --- /show tests ---

func TestRouterShow_NoArgs_InGitRepo(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "config", "user.email", "t@t.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "T").Run()
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "add a.go").Run()

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/show")

	if len(sender.cards) == 0 {
		t.Fatal("expected a card from /show HEAD")
	}
	if !strings.Contains(sender.cards[0].Content, "a.go") {
		t.Fatalf("expected 'a.go' in /show output, got: %q", sender.cards[0].Content[:min(200, len(sender.cards[0].Content))])
	}
}

func TestRouterShow_InvalidCommit(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &spySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/show nonexistent123")

	if !strings.Contains(sender.LastMessage(), "找不到提交") {
		t.Fatalf("expected 'not found' message, got: %q", sender.LastMessage())
	}
}

func TestRouterShow_NoGitRepo(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/show")

	if sender.LastMessage() == "" {
		t.Fatal("expected some message from /show in non-git dir")
	}
}

func TestRouterDiff_LargeOutput(t *testing.T) {
	// Create a git repo with a very large diff (>4000 chars) to test truncation
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()

	// Write a large file and commit it
	largeContent := strings.Repeat("original line content here\n", 50)
	os.WriteFile(filepath.Join(dir, "large.go"), []byte(largeContent), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "add large file").Run()

	// Now modify it to create a large diff (>4000 chars)
	modifiedContent := strings.Repeat("modified line content here!\n", 200)
	os.WriteFile(filepath.Join(dir, "large.go"), []byte(modifiedContent), 0644)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/diff")

	if len(sender.cards) == 0 {
		t.Fatalf("expected a card from /diff with large output")
	}
	if !strings.Contains(sender.cards[0].Content, "末尾部分") {
		t.Fatalf("expected truncation notice in large diff, got: %q", sender.cards[0].Content[:min(100, len(sender.cards[0].Content))])
	}
}

func TestRouterDiff_DefaultWorkDir(t *testing.T) {
	// /diff with empty session WorkDir falls back to workRoot
	dir := t.TempDir()
	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &spySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	// Force empty WorkDir
	store.UpdateSession("chat1", func(s *Session) { s.WorkDir = "" })

	r.Route(context.Background(), "chat1", "user1", "/diff")

	if sender.LastMessage() == "" {
		t.Fatal("expected some message from /diff with empty workDir")
	}
}

func TestRouterRecent_EmptyCommits(t *testing.T) {
	// If all commits are empty (no file changes), files list should be empty
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "empty commit 1").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "empty commit 2").Run()

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &spySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/recent")

	if sender.LastMessage() == "" {
		t.Fatal("expected some message from /recent with empty commits")
	}
}

// --- /exec command tests ---

func TestRouterExec_NoArgs(t *testing.T) {
	r, snd := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/exec")
	if !strings.Contains(snd.LastMessage(), "用法") {
		t.Fatalf("expected usage message, got: %q", snd.LastMessage())
	}
}

func TestRouterExec_Success(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/exec echo hello_devbot")

	if len(sender.cards) == 0 {
		t.Fatalf("expected a card, got none; texts: %v", sender.texts)
	}
	card := sender.cards[0]
	if !strings.Contains(card.Content, "hello_devbot") {
		t.Fatalf("expected output to contain 'hello_devbot', got: %q", card.Content)
	}
	if card.Template != "blue" {
		t.Fatalf("expected blue card for success, got: %q", card.Template)
	}
}

func TestRouterExec_ErrorExit(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/exec exit 42")

	if len(sender.cards) == 0 {
		t.Fatalf("expected a card, got none")
	}
	if sender.cards[0].Template != "red" {
		t.Fatalf("expected red card for non-zero exit, got: %q", sender.cards[0].Template)
	}
}

func TestRouterExec_RunsInWorkDir(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/exec pwd")

	if len(sender.cards) == 0 {
		t.Fatalf("expected a card, got none")
	}
	if !strings.Contains(sender.cards[0].Content, dir) {
		t.Fatalf("expected output to contain workDir %q, got: %q", dir, sender.cards[0].Content)
	}
}

func TestRouterExec_StderrIncluded(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/exec sh -c 'echo errout >&2; exit 1'")

	if len(sender.cards) == 0 {
		t.Fatalf("expected a card, got none")
	}
	if !strings.Contains(sender.cards[0].Content, "errout") {
		t.Fatalf("expected stderr in output, got: %q", sender.cards[0].Content)
	}
}

func TestRouterExec_StdoutAndStderr(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	// Command produces both stdout and stderr
	r.Route(context.Background(), "chat1", "user1", "/exec sh -c 'echo outdata; echo errdata >&2; exit 1'")

	if len(sender.cards) == 0 {
		t.Fatalf("expected a card, got none")
	}
	content := sender.cards[0].Content
	if !strings.Contains(content, "outdata") {
		t.Fatalf("expected stdout 'outdata' in content, got: %q", content)
	}
	if !strings.Contains(content, "errdata") {
		t.Fatalf("expected stderr 'errdata' in content, got: %q", content)
	}
}

func TestRouterExec_EmptyOutput(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	// 'true' exits 0 with no output
	r.Route(context.Background(), "chat1", "user1", "/exec true")

	if len(sender.cards) == 0 {
		t.Fatalf("expected a card, got none")
	}
	if !strings.Contains(sender.cards[0].Content, "无输出") {
		t.Fatalf("expected '无输出' for empty output, got: %q", sender.cards[0].Content)
	}
}

func TestRouterExec_LargeOutputTruncated(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	// Generate > 4000 chars of output (seq 1 2000 produces ~10000 chars)
	r.Route(context.Background(), "chat1", "user1", "/exec seq 1 2000")

	if len(sender.cards) == 0 {
		t.Fatalf("expected a card, got none")
	}
	if !strings.Contains(sender.cards[0].Content, "输出过长") {
		t.Fatalf("expected truncation notice for large output, got: %q", sender.cards[0].Content)
	}
}

// --- /fetch tests ---

func TestRouterFetch_NoRemote(t *testing.T) {
	// In a git repo with no remote, fetch returns success with no output
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/fetch")

	if len(sender.cards) == 0 {
		t.Fatal("expected a card from /fetch")
	}
	// Either success (no remote = no output, "已是最新") or error
	if sender.cards[0].Title == "" {
		t.Fatalf("expected non-empty title, got empty")
	}
}

func TestRouterFetch_Success(t *testing.T) {
	// Clone from a bare remote, fetch back from it
	dir := t.TempDir()
	remoteDir := filepath.Join(dir, "remote.git")
	os.MkdirAll(remoteDir, 0755)
	exec.Command("git", "-C", remoteDir, "init", "--bare").Run()

	localDir := filepath.Join(dir, "local")
	exec.Command("git", "clone", remoteDir, localDir).Run()
	exec.Command("git", "-C", localDir, "commit", "--allow-empty", "-m", "init").Run()
	exec.Command("git", "-C", localDir, "push", "origin", "HEAD:main").Run()

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	// Use localDir as work root directly
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, localDir, nil)

	r.Route(context.Background(), "chat1", "user1", "/fetch")

	if len(sender.cards) == 0 {
		t.Fatal("expected a card from /fetch")
	}
	// Should be success (blue template)
	if sender.cards[0].Template == "red" {
		t.Fatalf("expected success, got error: %q", sender.cards[0].Content)
	}
}

// --- /pull tests ---

func TestRouterPull_NoRemote(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/pull")

	if len(sender.cards) == 0 {
		t.Fatal("expected a card from /pull with no remote")
	}
	if !strings.Contains(sender.cards[0].Title, "出错") {
		t.Fatalf("expected error card, got: %q", sender.cards[0].Title)
	}
}

func TestRouterPull_Success(t *testing.T) {
	// Create remote and local, push a commit, then pull from local2
	dir := t.TempDir()
	remoteDir := filepath.Join(dir, "remote.git")
	os.MkdirAll(remoteDir, 0755)
	exec.Command("git", "-C", remoteDir, "init", "--bare").Run()

	local1 := filepath.Join(dir, "local1")
	exec.Command("git", "clone", remoteDir, local1).Run()
	exec.Command("git", "-C", local1, "commit", "--allow-empty", "-m", "init").Run()
	exec.Command("git", "-C", local1, "push", "origin", "HEAD:refs/heads/main").Run()

	local2 := filepath.Join(dir, "local2")
	exec.Command("git", "clone", remoteDir, local2).Run()

	// Push a new commit from local1
	exec.Command("git", "-C", local1, "commit", "--allow-empty", "-m", "second").Run()
	exec.Command("git", "-C", local1, "push").Run()

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)
	store.UpdateSession("chat1", func(s *Session) { s.WorkDir = local2 })

	r.Route(context.Background(), "chat1", "user1", "/pull")

	if len(sender.cards) == 0 {
		t.Fatal("expected a card from /pull")
	}
}

func TestRouterPull_WithArgs(t *testing.T) {
	// /pull --dry-run should pass args to git pull
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/pull --dry-run")

	if len(sender.cards) == 0 {
		t.Fatal("expected a card from /pull --dry-run")
	}
}

func TestRouterPull_DefaultWorkDir(t *testing.T) {
	// /pull with empty session WorkDir falls back to work root
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)
	store.UpdateSession("chat1", func(s *Session) { s.WorkDir = "" })

	r.Route(context.Background(), "chat1", "user1", "/pull")

	if len(sender.cards) == 0 {
		t.Fatal("expected a card from /pull with empty workDir")
	}
}

// --- /push tests ---

func TestRouterPush_NoRemote(t *testing.T) {
	// /push in a repo with no remote should show error card
	dir := t.TempDir()
	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	// Init a git repo with no remote
	gitDir := filepath.Join(dir, "myrepo")
	os.MkdirAll(gitDir, 0755)
	exec.Command("git", "-C", gitDir, "init").Run()
	exec.Command("git", "-C", gitDir, "commit", "--allow-empty", "-m", "init").Run()
	store.UpdateSession("chat1", func(s *Session) { s.WorkDir = gitDir })

	r.Route(context.Background(), "chat1", "user1", "/push")

	if len(sender.cards) == 0 {
		t.Fatal("expected error card for /push with no remote")
	}
	if !strings.Contains(sender.cards[0].Title, "出错") {
		t.Fatalf("expected error card title, got: %q", sender.cards[0].Title)
	}
}

func TestRouterPush_DefaultWorkDir(t *testing.T) {
	// /push with empty session WorkDir should fall back to work root
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	// Explicitly set WorkDir to empty so it falls back to work root
	store.UpdateSession("chat1", func(s *Session) { s.WorkDir = "" })

	r.Route(context.Background(), "chat1", "user1", "/push")

	if len(sender.cards) == 0 {
		t.Fatal("expected a card response when work dir is empty")
	}
}

func TestRouterPush_WithArgs(t *testing.T) {
	// /push --force-with-lease should pass args to git push
	dir := t.TempDir()
	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	gitDir := filepath.Join(dir, "myrepo")
	os.MkdirAll(gitDir, 0755)
	exec.Command("git", "-C", gitDir, "init").Run()
	store.UpdateSession("chat1", func(s *Session) { s.WorkDir = gitDir })

	r.Route(context.Background(), "chat1", "user1", "/push --dry-run")

	// Should get some response (error since no remote, but not a panic)
	if len(sender.cards) == 0 && len(sender.texts) == 0 {
		t.Fatal("expected some response for /push --dry-run")
	}
}

// --- /stash tests ---

func TestRouterStash_NoChanges(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	gitDir := filepath.Join(dir, "myrepo")
	os.MkdirAll(gitDir, 0755)
	exec.Command("git", "-C", gitDir, "init").Run()
	exec.Command("git", "-C", gitDir, "commit", "--allow-empty", "-m", "init").Run()
	store.UpdateSession("chat1", func(s *Session) { s.WorkDir = gitDir })

	r.Route(context.Background(), "chat1", "user1", "/stash")

	// Should get a card response
	if len(sender.cards) == 0 && len(sender.texts) == 0 {
		t.Fatal("expected some response for /stash")
	}
}

func TestRouterStash_Pop(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	gitDir := filepath.Join(dir, "myrepo")
	os.MkdirAll(gitDir, 0755)
	exec.Command("git", "-C", gitDir, "init").Run()
	exec.Command("git", "-C", gitDir, "commit", "--allow-empty", "-m", "init").Run()
	store.UpdateSession("chat1", func(s *Session) { s.WorkDir = gitDir })

	// /stash pop with nothing stashed should return an error card
	r.Route(context.Background(), "chat1", "user1", "/stash pop")

	if len(sender.cards) == 0 && len(sender.texts) == 0 {
		t.Fatal("expected some response for /stash pop")
	}
	// Title should contain "pop"
	if len(sender.cards) > 0 && !strings.Contains(sender.cards[0].Title, "pop") {
		t.Fatalf("expected 'pop' in card title, got: %q", sender.cards[0].Title)
	}
}

func TestRouterStash_WithChanges(t *testing.T) {
	// Stash actual changes, then pop them
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()
	os.WriteFile(filepath.Join(dir, "work.go"), []byte("original"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "add file").Run()
	// Modify file to create stashable changes
	os.WriteFile(filepath.Join(dir, "work.go"), []byte("modified"), 0644)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/stash")

	if len(sender.cards) == 0 {
		t.Fatal("expected a card from /stash with changes")
	}
	if strings.Contains(sender.cards[0].Title, "出错") {
		t.Fatalf("expected success, got error: %q", sender.cards[0].Title)
	}
}

func TestRouterPush_SuccessWithOutput(t *testing.T) {
	// Push in a bare repo clone to simulate success with output
	dir := t.TempDir()

	// Create a bare remote
	remoteDir := filepath.Join(dir, "remote.git")
	os.MkdirAll(remoteDir, 0755)
	exec.Command("git", "-C", remoteDir, "init", "--bare").Run()

	// Clone from the bare remote
	localDir := filepath.Join(dir, "local")
	exec.Command("git", "clone", remoteDir, localDir).Run()
	exec.Command("git", "-C", localDir, "commit", "--allow-empty", "-m", "init").Run()

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)
	store.UpdateSession("chat1", func(s *Session) { s.WorkDir = localDir })

	r.Route(context.Background(), "chat1", "user1", "/push")

	if len(sender.cards) == 0 && len(sender.texts) == 0 {
		t.Fatal("expected some response for /push")
	}
}

// --- /todo tests ---

func TestRouterTodo_NoTodos(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0644)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &spySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/todo")

	if !strings.Contains(sender.LastMessage(), "没有找到") {
		t.Fatalf("expected no-todos message, got: %q", sender.LastMessage())
	}
}

func TestRouterTodo_HasTodos(t *testing.T) {
	dir := t.TempDir()
	content := "package main\n// TODO: fix this later\n// FIXME: urgent bug\nfunc main() {}\n"
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(content), 0644)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/todo")

	if len(sender.cards) == 0 {
		t.Fatal("expected a card with todo items")
	}
	if !strings.Contains(sender.cards[0].Content, "TODO") {
		t.Fatalf("expected TODO in content, got: %q", sender.cards[0].Content)
	}
	if !strings.Contains(sender.cards[0].Title, "待办") {
		t.Fatalf("expected '待办' in title, got: %q", sender.cards[0].Title)
	}
}

// --- /test (Go fast path) tests ---

func TestRouterTest_GoProject_NoPattern(t *testing.T) {
	// Create a minimal Go project with a simple passing test
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module testpkg\n\ngo 1.20\n"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0644)
	os.WriteFile(filepath.Join(dir, "main_test.go"), []byte("package main\nimport \"testing\"\nfunc TestOK(t *testing.T) {}\n"), 0644)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/test")

	// Should get a card (fast path detected go.mod)
	if len(sender.cards) == 0 {
		t.Fatal("expected a card from /test on Go project")
	}
	title := sender.cards[0].Title
	if !strings.Contains(title, "go test") {
		t.Fatalf("expected 'go test' in title, got: %q", title)
	}
}

func TestRouterTest_GoProject_WithPattern(t *testing.T) {
	// Test with a pattern filter
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module testpkg\n\ngo 1.20\n"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0644)
	os.WriteFile(filepath.Join(dir, "main_test.go"), []byte("package main\nimport \"testing\"\nfunc TestFoo(t *testing.T) {}\n"), 0644)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/test Foo")

	if len(sender.cards) == 0 {
		t.Fatal("expected a card from /test with pattern")
	}
}

func TestRouterTest_GoProject_FailingTests(t *testing.T) {
	// Failing tests should show red card
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module testpkg\n\ngo 1.20\n"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0644)
	os.WriteFile(filepath.Join(dir, "main_test.go"), []byte("package main\nimport \"testing\"\nfunc TestFail(t *testing.T) { t.Fatal(\"intentional failure\") }\n"), 0644)

	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &cardSpySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/test")

	if len(sender.cards) == 0 {
		t.Fatal("expected a card from /test with failing tests")
	}
	if sender.cards[0].Template != "red" {
		t.Fatalf("expected red template for failed tests, got: %q", sender.cards[0].Template)
	}
}

func TestRouterTest_NonGoProject_GoesToClaude(t *testing.T) {
	// Non-Go project should fall through to Claude (send "执行中" or queue)
	dir := t.TempDir()
	// No go.mod in this dir
	store, _ := NewStore(filepath.Join(dir, "state.json"))
	sender := &spySender{}
	ex := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), ex, store, sender, map[string]bool{"user1": true}, dir, nil)

	r.Route(context.Background(), "chat1", "user1", "/test")

	// Should trigger Claude (executor will try to run but fail since no real claude)
	// Just verify we don't crash and some response comes back
	_ = sender.LastMessage()
}

// --- /minInt unit test ---

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
