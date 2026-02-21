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

	// Mutating returned copy does NOT affect store
	sess.Model = "opus"
	sess2 := s.GetSession("chat1", "/other", "haiku")
	if sess2.Model != "sonnet" {
		t.Fatalf("expected existing session with Model sonnet (unchanged), got %q", sess2.Model)
	}

	// UpdateSession mutates the stored session
	s.UpdateSession("chat1", func(s *Session) {
		s.Model = "opus"
	})
	sess3 := s.GetSession("chat1", "/other", "haiku")
	if sess3.Model != "opus" {
		t.Fatalf("expected Model opus after UpdateSession, got %q", sess3.Model)
	}
}

func TestStoreCorruptJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	os.WriteFile(path, []byte("not valid json {{{"), 0644)

	_, err := NewStore(path)
	if err == nil {
		t.Fatalf("expected error for corrupt JSON state file")
	}
}

func TestStoreNewStore_ReadDirError(t *testing.T) {
	// Passing a directory path (not a file) should trigger a non-IsNotExist read error.
	dir := t.TempDir()
	_, err := NewStore(dir) // dir itself is passed as the state file path
	if err == nil {
		t.Fatalf("expected error when path is a directory")
	}
}

func TestStoreSave_WriteError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; permission-based tests are unreliable")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	s.SetWorkRoot("/test")

	// Make the parent directory non-writable so WriteFile fails.
	os.Chmod(dir, 0555)
	defer os.Chmod(dir, 0755)

	if err := s.Save(); err == nil {
		t.Fatalf("expected error when directory is not writable")
	}
}

func TestStoreSave_MkdirError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	s.SetWorkRoot("/test")

	// Redirect path to a location where the parent dir cannot be created.
	// On macOS/Linux, creating directories under "/" requires root.
	s.path = "/nonexistent_root_xyz_test/subdir/state.json"
	if err := s.Save(); err == nil {
		t.Fatalf("expected error when parent dir cannot be created")
	}
}

func TestNewStore_ExistingFileNilMaps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	// Write valid JSON with null maps — json.Unmarshal sets map fields to nil,
	// triggering the re-initialization branches in NewStore.
	os.WriteFile(path, []byte(`{"chats":null,"docBindings":null,"workRoot":"/some/path"}`), 0644)

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.state.Chats == nil {
		t.Fatalf("expected Chats to be initialized to non-nil map")
	}
	if s.state.DocBindings == nil {
		t.Fatalf("expected DocBindings to be initialized to non-nil map")
	}
}

func TestStoreSessionExecParams_NoSession(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	workDir, sessionID, permMode, model := s.SessionExecParams("nonexistent_chat")
	if workDir != "" || sessionID != "" || permMode != "" || model != "" {
		t.Fatalf("expected empty params for nonexistent session, got: %q %q %q %q",
			workDir, sessionID, permMode, model)
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
