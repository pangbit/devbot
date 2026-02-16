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
	if s.State().WorkRoot != "" {
		t.Fatalf("expected empty WorkRoot, got %q", s.State().WorkRoot)
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

	s.State().WorkRoot = "/tmp/work"
	s.State().Chats["chat1"] = &Session{
		ClaudeSessionID: "ses1",
		WorkDir:         "/tmp/work/proj",
		Model:           "opus",
	}
	s.State().DocBindings["README.md"] = "docx_abc"

	if err := s.Save(); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	s2, err := NewStore(path)
	if err != nil {
		t.Fatalf("reload error: %v", err)
	}
	if s2.State().WorkRoot != "/tmp/work" {
		t.Fatalf("WorkRoot mismatch after reload")
	}
	chat := s2.State().Chats["chat1"]
	if chat == nil || chat.ClaudeSessionID != "ses1" || chat.Model != "opus" {
		t.Fatalf("chat session mismatch after reload")
	}
	if s2.State().DocBindings["README.md"] != "docx_abc" {
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
	s.State().WorkRoot = "/test"
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
