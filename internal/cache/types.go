package cache

import (
	"time"

	"github.com/petems/pr-wrangler/internal/github"
)

// CacheEntry holds a cached PR list for a single query, alongside metadata
// needed to detect staleness or replay SAML errors.
type CacheEntry struct {
	PRs         []github.PR            `json:"prs"`
	Query       string                 `json:"query"`
	LastUpdated time.Time              `json:"last_updated"`
	SAMLErrors  []CachedSAMLErrorEntry `json:"saml_errors,omitempty"`
}

// CachedSAMLErrorEntry is the on-disk representation of a SAMLErrorEntry.
// The runtime type carries an `error` interface (OriginalError) which JSON
// cannot round-trip, so the cache stores only the fields needed to rebuild
// the placeholder row on load.
type CachedSAMLErrorEntry struct {
	Index             int    `json:"index"`
	RepoNameWithOwner string `json:"repo_name_with_owner,omitempty"`
	PRNumber          int    `json:"pr_number,omitempty"`
	Message           string `json:"message,omitempty"`
	AuthURL           string `json:"auth_url,omitempty"`
}

// ToSAMLErrorEntries rebuilds runtime SAMLErrorEntry values from their
// serializable form. OriginalError is left nil — it isn't read by the
// placeholder renderer and isn't safe to round-trip through JSON.
func ToSAMLErrorEntries(cached []CachedSAMLErrorEntry) []github.SAMLErrorEntry {
	if len(cached) == 0 {
		return nil
	}
	out := make([]github.SAMLErrorEntry, len(cached))
	for i, c := range cached {
		out[i] = github.SAMLErrorEntry{
			Index:             c.Index,
			RepoNameWithOwner: c.RepoNameWithOwner,
			PRNumber:          c.PRNumber,
			Err: &github.SAMLAuthError{
				Message: c.Message,
				AuthURL: c.AuthURL,
			},
		}
	}
	return out
}

// fromSAMLErrorEntries flattens runtime entries to the serializable cache
// shape, dropping the non-serializable OriginalError field.
func fromSAMLErrorEntries(entries []github.SAMLErrorEntry) []CachedSAMLErrorEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]CachedSAMLErrorEntry, len(entries))
	for i, e := range entries {
		ce := CachedSAMLErrorEntry{
			Index:             e.Index,
			RepoNameWithOwner: e.RepoNameWithOwner,
			PRNumber:          e.PRNumber,
		}
		if e.Err != nil {
			ce.Message = e.Err.Message
			ce.AuthURL = e.Err.AuthURL
		}
		out[i] = ce
	}
	return out
}

// Cache persists per-query PR results to disk as JSON. The file path is
// configurable so callers (and tests) can decide where the cache lives.
type Cache struct {
	Path    string                `json:"-"`
	Entries map[string]CacheEntry `json:"entries"`
}
