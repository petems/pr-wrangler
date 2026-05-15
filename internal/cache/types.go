package cache

import (
	"time"

	"github.com/petems/pr-wrangler/internal/github"
)

// CacheEntry holds a cached PR list for a single query, alongside metadata
// needed to detect staleness or replay SAML errors.
type CacheEntry struct {
	PRs         []github.PR             `json:"prs"`
	Query       string                  `json:"query"`
	LastUpdated time.Time               `json:"last_updated"`
	SAMLErrors  []github.SAMLErrorEntry `json:"saml_errors,omitempty"`
}

// Cache persists per-query PR results to disk as JSON. The file path is
// configurable so callers (and tests) can decide where the cache lives.
type Cache struct {
	Path    string                `json:"-"`
	Entries map[string]CacheEntry `json:"entries"`
}
