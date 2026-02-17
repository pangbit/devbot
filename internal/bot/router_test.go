package bot

import (
	"context"
	"fmt"
	"os"
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

func TestRouterStatus(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/status")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "Uptime") || !strings.Contains(msg, "sonnet") {
		t.Fatalf("status should show Uptime and model, got: %q", msg)
	}
	if !strings.Contains(msg, "Execs:") {
		t.Fatalf("status should show Execs, got: %q", msg)
	}
	if !strings.Contains(msg, "LastExec:") {
		t.Fatalf("status should show LastExec, got: %q", msg)
	}
	if !strings.Contains(msg, "Queued:") {
		t.Fatalf("status should show Queued, got: %q", msg)
	}
	// With no executions, LastExec should be "-"
	if !strings.Contains(msg, "LastExec:** -") {
		t.Fatalf("status should show LastExec: - when no execs, got: %q", msg)
	}
	// Execs should be 0
	if !strings.Contains(msg, "Execs:**    0") {
		t.Fatalf("status should show Execs: 0, got: %q", msg)
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
	if !strings.Contains(msg, "Changed to") || !strings.Contains(msg, "project1") {
		t.Fatalf("expected cd confirmation, got: %q", msg)
	}
}

func TestRouterCdInvalid(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/cd nonexistent")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "not found") {
		t.Fatalf("expected not found error, got: %q", msg)
	}
}

func TestRouterNewSession(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/new")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "New session") {
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
	if !strings.Contains(msg, "YOLO") {
		t.Fatalf("expected yolo msg, got: %q", msg)
	}
	r.Route(context.Background(), "chat1", "user1", "/safe")
	msg = sender.LastMessage()
	if !strings.Contains(msg, "Safe") {
		t.Fatalf("expected safe msg, got: %q", msg)
	}
}

func TestRouterLast_NoOutput(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/last")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "No previous") {
		t.Fatalf("expected no previous output msg, got: %q", msg)
	}
}

func TestRouterKill_NoProcess(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/kill")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "No running") {
		t.Fatalf("expected no running task msg, got: %q", msg)
	}
}

func TestRouterUnknownCommand(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/unknown")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "Unknown command") {
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

func TestRouterFileNotFound(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/file nonexistent.txt")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "not found") {
		t.Fatalf("expected not found, got: %q", msg)
	}
}

func TestRouterCdPathTraversal(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/cd ../../etc")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "Cannot cd outside") {
		t.Fatalf("expected path traversal rejection, got: %q", msg)
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
	if !strings.Contains(sender.messages[0], "Image saved to:") {
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
	if !strings.Contains(sender.messages[0], "File saved to:") {
		t.Fatalf("expected 'File saved to:' message, got: %q", sender.messages[0])
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
	if !strings.Contains(msg, "Current root:") {
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
	if !strings.Contains(msg, "Root set to:") {
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
	if !strings.Contains(msg, "absolute path") {
		t.Fatalf("expected absolute path error, got: %q", msg)
	}
}

func TestRouterRoot_RejectSystemDirs(t *testing.T) {
	r, sender := newTestRouter(t)
	for _, d := range []string{"/", "/etc", "/var", "/usr", "/sys", "/proc"} {
		r.Route(context.Background(), "chat1", "user1", "/root "+d)
		msg := sender.LastMessage()
		if !strings.Contains(msg, "system directory") {
			t.Fatalf("expected system dir rejection for %s, got: %q", d, msg)
		}
	}
}

func TestRouterRoot_RejectNonExistent(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/root /nonexistent_path_xyz_abc")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "not found") {
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
	if !strings.Contains(msg, "Not a directory") {
		t.Fatalf("expected not a directory, got: %q", msg)
	}
}

// --- /sessions tests ---

func TestRouterSessions_Empty(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/sessions")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "No sessions") {
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
	if !strings.Contains(msg, "current-sess") || !strings.Contains(msg, "(current)") {
		t.Fatalf("expected current session, got: %q", msg)
	}
}

// --- /switch tests ---

func TestRouterSwitch_EmptyArgs(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/switch")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "Usage:") {
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
	if !strings.Contains(msg, "Switched to session: new-sess") {
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
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/commit")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "Usage:") {
		t.Fatalf("expected usage, got: %q", msg)
	}
}

func TestRouterSh_EmptyArgs(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/sh")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "Usage:") {
		t.Fatalf("expected usage, got: %q", msg)
	}
}

func TestRouterSummary_NoOutput(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/summary")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "No previous output") {
		t.Fatalf("expected no output msg, got: %q", msg)
	}
}

func TestRouterModelShowCurrent(t *testing.T) {
	r, sender := newTestRouter(t)
	r.Route(context.Background(), "chat1", "user1", "/model")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "Current model:") {
		t.Fatalf("expected current model, got: %q", msg)
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
			hasExecuting := false
			for _, m := range msgs {
				if strings.Contains(m, "Executing...") {
					hasExecuting = true
				}
			}
			if !hasExecuting {
				t.Fatalf("expected 'Executing...' for %s, got: %v", tc.cmd, msgs)
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
	hasExecuting := false
	for _, m := range msgs {
		if strings.Contains(m, "Executing...") {
			hasExecuting = true
		}
	}
	if !hasExecuting {
		t.Fatalf("expected 'Executing...' message, got: %v", msgs)
	}
}

func TestRouterHandlePrompt(t *testing.T) {
	r, sender, q := newTestRouterForExec(t)
	r.Route(context.Background(), "chat1", "user1", "hello Claude")
	q.Shutdown()
	msgs := sender.Messages()
	hasExecuting := false
	for _, m := range msgs {
		if strings.Contains(m, "Executing...") {
			hasExecuting = true
		}
	}
	if !hasExecuting {
		t.Fatalf("expected 'Executing...' message, got: %v", msgs)
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
	if !strings.Contains(sender.messages[0], "Executing...") {
		t.Fatalf("expected 'Executing...', got: %q", sender.messages[0])
	}
	if !strings.Contains(sender.messages[1], "Error") {
		t.Fatalf("expected error message, got: %q", sender.messages[1])
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
		if strings.Contains(m, "Queued") {
			hasQueued = true
		}
	}
	if !hasQueued {
		t.Fatalf("expected 'Queued' message, got: %v", msgs)
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

	if len(sender.cards) < 2 {
		t.Fatalf("expected at least 2 cards, got %d: %+v", len(sender.cards), sender.cards)
	}
	// First card: Executing...
	if sender.cards[0].Title != "Executing..." {
		t.Fatalf("expected Executing... card, got: %q", sender.cards[0].Title)
	}
	// Last card: response (no title)
	last := sender.cards[len(sender.cards)-1]
	if last.Title != "" {
		t.Fatalf("expected no title on response card, got: %q", last.Title)
	}
	if !strings.Contains(last.Content, "**bold**") {
		t.Fatalf("expected markdown content, got: %q", last.Content)
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
		if c.Title == "Error" && c.Template == "red" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected red Error card, got cards: %+v texts: %v", sender.cards, sender.texts)
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

	// Should have: Executing... card, then final result card
	if len(sender.cards) < 2 {
		t.Fatalf("expected at least 2 cards, got %d: %+v", len(sender.cards), sender.cards)
	}
	// First card: Executing...
	if sender.cards[0].Title != "Executing..." {
		t.Fatalf("expected Executing..., got: %q", sender.cards[0].Title)
	}
	// Last card: final result
	last := sender.cards[len(sender.cards)-1]
	if !strings.Contains(last.Content, "Final answer") {
		t.Fatalf("expected final answer in last card, got: %q", last.Content)
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
