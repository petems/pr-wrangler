package cache

import (
	"path/filepath"
	"testing"

	"github.com/petems/pr-wrangler/internal/github"
)

// TestSaveLoadRoundTripWithSAMLErrors guards against the regression where
// CacheEntry.SAMLErrors held github.SAMLErrorEntry directly: SAMLAuthError
// carries an `error` interface (OriginalError) that JSON cannot unmarshal
// back, so Load would fail and the whole cache would be discarded.
func TestSaveLoadRoundTripWithSAMLErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pr-cache.json")

	c := NewCache(path)
	c.SetForQuery("q", []github.PR{{Number: 1, Title: "ok"}}, []github.SAMLErrorEntry{
		{
			Index:             2,
			RepoNameWithOwner: "example/private",
			PRNumber:          42,
			Err: &github.SAMLAuthError{
				Message:       "Resource protected by SSO",
				AuthURL:       "https://github.com/orgs/example/sso",
				OriginalError: assertableError{},
			},
		},
	})

	if err := c.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded := NewCache(path)
	if err := reloaded.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	entry, ok := reloaded.GetForQuery("q")
	if !ok {
		t.Fatalf("expected query entry to be present")
	}
	if len(entry.SAMLErrors) != 1 {
		t.Fatalf("expected 1 SAML error, got %d", len(entry.SAMLErrors))
	}
	got := entry.SAMLErrors[0]
	if got.Index != 2 || got.RepoNameWithOwner != "example/private" || got.PRNumber != 42 {
		t.Fatalf("unexpected SAML entry metadata: %+v", got)
	}
	if got.AuthURL != "https://github.com/orgs/example/sso" || got.Message != "Resource protected by SSO" {
		t.Fatalf("unexpected SAML entry payload: %+v", got)
	}

	rebuilt := ToSAMLErrorEntries(entry.SAMLErrors)
	if len(rebuilt) != 1 || rebuilt[0].Err == nil || rebuilt[0].Err.AuthURL == "" {
		t.Fatalf("rebuild lost SAML auth metadata: %+v", rebuilt)
	}
}

type assertableError struct{}

func (assertableError) Error() string { return "boom" }
