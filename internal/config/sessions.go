package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SessionType distinguishes local work sessions from PR-attached sessions.
type SessionType string

const (
	SessionTypePR    SessionType = "pr"
	SessionTypeLocal SessionType = "local"
)

// SessionEntry tracks a single dashboard-managed tmux session.
type SessionEntry struct {
	TmuxSession  string      `json:"tmux_session"`
	Repo         string      `json:"repo"`
	WorktreePath string      `json:"worktree_path"`
	Branch       string      `json:"branch"`
	PRNumber     int         `json:"pr_number"`
	Type         SessionType `json:"type"`
}

// SessionState is the root object persisted to the sessions file.
type SessionState struct {
	Sessions []SessionEntry `json:"sessions"`
}

// SessionsPath returns the path to the sessions state file.
func SessionsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("finding config dir: %w", err)
	}
	return filepath.Join(dir, "pr-wrangler", "sessions.json"), nil
}

// LoadSessions reads and parses the session state file. Returns empty state
// (not error) if the file is missing or unparseable.
func LoadSessions(path string) SessionState {
	data, err := os.ReadFile(path)
	if err != nil {
		return SessionState{}
	}

	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return SessionState{}
	}
	return state
}

// SaveSessions writes the session state to disk, creating the directory if needed.
func SaveSessions(state SessionState, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating sessions dir: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling sessions: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing sessions: %w", err)
	}
	return nil
}
