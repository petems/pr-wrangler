package github

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RequiredScopes are the GitHub OAuth scopes pr-wrangler needs.
// - repo: read/write access to PRs, issues, comments, statuses
var RequiredScopes = []string{"repo"}

// TokenInfo holds a stored GitHub token with metadata.
type TokenInfo struct {
	Token     string    `json:"token"`
	Scopes    []string  `json:"scopes"`
	User      string    `json:"user,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

// IsExpired reports whether the token has expired.
func (t *TokenInfo) IsExpired() bool {
	if t.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(t.ExpiresAt)
}

// authFilePath returns the path to the auth file (~/.config/pr-wrangler/auth.json).
func authFilePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("finding config dir: %w", err)
	}
	return filepath.Join(dir, "pr-wrangler", "auth.json"), nil
}

// LoadToken loads a stored token from disk.
// Returns nil (no error) if no token file exists.
func LoadToken() (*TokenInfo, error) {
	path, err := authFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading auth file: %w", err)
	}

	var info TokenInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("parsing auth file: %w", err)
	}

	return &info, nil
}

// SaveToken persists token info to disk.
func SaveToken(info *TokenInfo) error {
	path, err := authFilePath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling token: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing auth file: %w", err)
	}

	return nil
}

// ClearToken removes the stored token.
func ClearToken() error {
	path, err := authFilePath()
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing auth file: %w", err)
	}
	return nil
}

// ResolveToken returns a GitHub token, checking in order:
// 1. GITHUB_TOKEN env var
// 2. GH_TOKEN env var
// 3. Stored pr-wrangler token (~/.config/pr-wrangler/auth.json)
//
// Returns the token and its source description, or empty string if none found.
func ResolveToken() (token string, source string, err error) {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t, "GITHUB_TOKEN env var", nil
	}

	if t := os.Getenv("GH_TOKEN"); t != "" {
		return t, "GH_TOKEN env var", nil
	}

	info, err := LoadToken()
	if err != nil {
		return "", "", fmt.Errorf("loading stored token: %w", err)
	}

	if info != nil {
		if info.IsExpired() {
			return "", "", fmt.Errorf("stored token has expired (run 'pr-wrangler auth login' to re-authenticate)")
		}
		return info.Token, "pr-wrangler auth", nil
	}

	return "", "", nil
}
