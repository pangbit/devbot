package bot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Session struct {
	ClaudeSessionID string            `json:"claudeSessionID,omitempty"`
	WorkDir         string            `json:"workDir,omitempty"`
	Model           string            `json:"model,omitempty"`
	PermissionMode  string            `json:"permissionMode,omitempty"`
	History         []string          `json:"history,omitempty"`
	LastOutput      string            `json:"lastOutput,omitempty"`
	LastPrompt      string            `json:"lastPrompt,omitempty"`
	DirSessions     map[string]string `json:"dirSessions,omitempty"`
}

type State struct {
	Chats       map[string]*Session `json:"chats"`
	DocBindings map[string]string   `json:"docBindings"`
	WorkRoot    string              `json:"workRoot,omitempty"`
}

type Store struct {
	mu    sync.RWMutex
	path  string
	state *State
}

func NewStore(path string) (*Store, error) {
	s := &Store{
		path: path,
		state: &State{
			Chats:       make(map[string]*Session),
			DocBindings: make(map[string]string),
		},
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, s.state); err != nil {
		return nil, err
	}
	if s.state.Chats == nil {
		s.state.Chats = make(map[string]*Session)
	}
	if s.state.DocBindings == nil {
		s.state.DocBindings = make(map[string]string)
	}
	return s, nil
}

// State returns the raw state pointer. Kept for backward compatibility in tests;
// production code should use the thread-safe accessors below.
func (s *Store) State() *State {
	return s.state
}

// GetSession returns a snapshot copy of the session for chatID, creating one
// if needed with defaults. The returned value is safe to read without locks.
// To mutate session fields, use UpdateSession.
func (s *Store) GetSession(chatID, defaultWorkDir, defaultModel string) Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := s.state.Chats[chatID]
	if sess == nil {
		sess = &Session{
			WorkDir: defaultWorkDir,
			Model:   defaultModel,
		}
		s.state.Chats[chatID] = sess
	}
	// Return a value copy — callers get a consistent snapshot
	cp := *sess
	cp.History = append([]string(nil), sess.History...)
	return cp
}

func (s *Store) WorkRoot() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.WorkRoot
}

func (s *Store) SetWorkRoot(root string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.WorkRoot = root
}

func (s *Store) DocBindings() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return a copy
	cp := make(map[string]string, len(s.state.DocBindings))
	for k, v := range s.state.DocBindings {
		cp[k] = v
	}
	return cp
}

func (s *Store) SetDocBinding(filePath, docID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.DocBindings[filePath] = docID
}

func (s *Store) RemoveDocBinding(filePath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.state.DocBindings, filePath)
}

// UpdateSession runs fn with the session for chatID under the write lock.
// The session must already exist (via GetSession).
func (s *Store) UpdateSession(chatID string, fn func(*Session)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.state.Chats[chatID]; ok {
		fn(sess)
	}
}

// SessionExecParams returns execution parameters for a session under read lock.
func (s *Store) SessionExecParams(chatID string) (workDir, sessionID, permMode, model string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess := s.state.Chats[chatID]
	if sess == nil {
		return
	}
	return sess.WorkDir, sess.ClaudeSessionID, sess.PermissionMode, sess.Model
}

func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
