package bot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Session struct {
	ClaudeSessionID string   `json:"claudeSessionID,omitempty"`
	WorkDir         string   `json:"workDir,omitempty"`
	Model           string   `json:"model,omitempty"`
	PermissionMode  string   `json:"permissionMode,omitempty"`
	History         []string `json:"history,omitempty"`
	LastOutput      string   `json:"lastOutput,omitempty"`
}

type State struct {
	Chats       map[string]*Session `json:"chats"`
	DocBindings map[string]string   `json:"docBindings"`
	WorkRoot    string              `json:"workRoot,omitempty"`
}

type Store struct {
	mu    sync.Mutex
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

func (s *Store) State() *State {
	return s.state
}

func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
