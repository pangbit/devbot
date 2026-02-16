package bot

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
	r := NewRouter(exec, store, sender, map[string]bool{"user1": true}, dir)
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
	if !strings.Contains(msg, "uptime") || !strings.Contains(msg, "sonnet") {
		t.Fatalf("status should show uptime and model, got: %q", msg)
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

func TestRouterWorkRootNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	store.SetWorkRoot("/existing/root")
	sender := &spySender{}
	exec := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	NewRouter(exec, store, sender, map[string]bool{"user1": true}, "/new/root")
	if store.WorkRoot() != "/existing/root" {
		t.Fatalf("expected WorkRoot to remain /existing/root, got %q", store.WorkRoot())
	}
}
