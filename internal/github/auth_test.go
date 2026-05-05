package github

import (
	"os"
	"testing"
	"time"
)

func TestSaveAndLoadToken(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	info := &TokenInfo{
		Token:     "test-token-123",
		Scopes:    []string{"repo"},
		User:      "testuser",
		CreatedAt: time.Now().Truncate(time.Second),
	}

	if err := SaveToken(info); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	authFile, err := authFilePath()
	if err != nil {
		t.Fatalf("authFilePath: %v", err)
	}
	stat, err := os.Stat(authFile)
	if err != nil {
		t.Fatalf("stat auth file: %v", err)
	}
	if got := stat.Mode().Perm(); got != 0o600 {
		t.Fatalf("auth file mode: got %v, want 0600", got)
	}

	loaded, err := LoadToken()
	if err != nil {
		t.Fatalf("LoadToken: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadToken returned nil")
	}

	if loaded.Token != info.Token {
		t.Errorf("Token: got %q, want %q", loaded.Token, info.Token)
	}
	if loaded.User != info.User {
		t.Errorf("User: got %q, want %q", loaded.User, info.User)
	}
	if len(loaded.Scopes) != 1 || loaded.Scopes[0] != "repo" {
		t.Errorf("Scopes: got %v, want [repo]", loaded.Scopes)
	}
}

func TestTokenInfo_IsExpired_ZeroExpiry(t *testing.T) {
	info := &TokenInfo{Token: "tok"}
	if info.IsExpired() {
		t.Error("zero ExpiresAt should not be expired")
	}
}

func TestTokenInfo_IsExpired_Past(t *testing.T) {
	info := &TokenInfo{
		Token:     "tok",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	if !info.IsExpired() {
		t.Error("past ExpiresAt should be expired")
	}
}

func TestTokenInfo_IsExpired_Future(t *testing.T) {
	info := &TokenInfo{
		Token:     "tok",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if info.IsExpired() {
		t.Error("future ExpiresAt should not be expired")
	}
}

func TestResolveToken_GITHUB_TOKEN(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "from-env")
	t.Setenv("GH_TOKEN", "other")

	token, source, err := ResolveToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "from-env" {
		t.Errorf("got %q, want 'from-env'", token)
	}
	if source != "GITHUB_TOKEN env var" {
		t.Errorf("source: got %q", source)
	}
}

func TestResolveToken_GH_TOKEN(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "from-gh")

	token, source, err := ResolveToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "from-gh" {
		t.Errorf("got %q, want 'from-gh'", token)
	}
	if source != "GH_TOKEN env var" {
		t.Errorf("source: got %q", source)
	}
}

func TestRequiredScopes(t *testing.T) {
	if len(RequiredScopes) == 0 {
		t.Error("RequiredScopes should not be empty")
	}
	if RequiredScopes[0] != "repo" {
		t.Errorf("first scope should be 'repo', got %q", RequiredScopes[0])
	}
}
