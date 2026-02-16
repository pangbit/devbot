package bot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreLoadEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	if s.WorkRoot() != "" {
		t.Fatalf("expected empty WorkRoot, got %q", s.WorkRoot())
	}
	if len(s.State().Chats) != 0 {
		t.Fatalf("expected no chats")
	}
}

func TestStoreSaveAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	s.SetWorkRoot("/tmp/work")
	s.State().Chats["chat1"] = &Session{
		ClaudeSessionID: "ses1",
		WorkDir:         "/tmp/work/proj",
		Model:           "opus",
	}
	s.SetDocBinding("README.md", "docx_abc")

	if err := s.Save(); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	s2, err := NewStore(path)
	if err != nil {
		t.Fatalf("reload error: %v", err)
	}
	if s2.WorkRoot() != "/tmp/work" {
		t.Fatalf("WorkRoot mismatch after reload")
	}
	chat := s2.State().Chats["chat1"]
	if chat == nil || chat.ClaudeSessionID != "ses1" || chat.Model != "opus" {
		t.Fatalf("chat session mismatch after reload")
	}
	bindings := s2.DocBindings()
	if bindings["README.md"] != "docx_abc" {
		t.Fatalf("DocBindings mismatch after reload")
	}
}

func TestStoreAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "state.json")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	s.SetWorkRoot("/test")
	if err := s.Save(); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("file should not be empty")
	}
}

func TestStoreGetSession(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	// GetSession creates a new session with defaults
	sess := s.GetSession("chat1", "/work", "sonnet")
	if sess.WorkDir != "/work" {
		t.Fatalf("expected WorkDir /work, got %q", sess.WorkDir)
	}
	if sess.Model != "sonnet" {
		t.Fatalf("expected Model sonnet, got %q", sess.Model)
	}

	// GetSession returns existing session
	sess.Model = "opus"
	sess2 := s.GetSession("chat1", "/other", "haiku")
	if sess2.Model != "opus" {
		t.Fatalf("expected existing session with Model opus, got %q", sess2.Model)
	}
}

func TestStoreDocBindings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	s.SetDocBinding("a.md", "doc1")
	s.SetDocBinding("b.md", "doc2")

	bindings := s.DocBindings()
	if bindings["a.md"] != "doc1" || bindings["b.md"] != "doc2" {
		t.Fatalf("unexpected bindings: %v", bindings)
	}

	// Verify it's a copy (mutating the returned map doesn't affect store)
	bindings["c.md"] = "doc3"
	bindings2 := s.DocBindings()
	if _, ok := bindings2["c.md"]; ok {
		t.Fatalf("DocBindings should return a copy")
	}

	s.RemoveDocBinding("a.md")
	bindings3 := s.DocBindings()
	if _, ok := bindings3["a.md"]; ok {
		t.Fatalf("expected a.md to be removed")
	}
}
