package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/petems/pr-wrangler/internal/github"
)

// maxEntries caps the number of distinct query results retained on disk so
// the cache file cannot grow without bound as users experiment with queries.
const maxEntries = 10

// NewCache returns a Cache backed by the given file path. The file is not
// touched until Load or Save is called.
func NewCache(path string) *Cache {
	return &Cache{
		Path:    path,
		Entries: make(map[string]CacheEntry),
	}
}

// Load reads and JSON-unmarshals the cache file. A missing file is not an
// error: the cache stays empty so first-run callers see the same shape as a
// returning user.
func (c *Cache) Load() error {
	data, err := os.ReadFile(c.Path) // #nosec G304 -- path is user's own cache file
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading PR cache: %w", err)
	}

	var loaded Cache
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("parsing PR cache: %w", err)
	}

	if loaded.Entries == nil {
		c.Entries = make(map[string]CacheEntry)
	} else {
		c.Entries = loaded.Entries
	}
	return nil
}

// Save JSON-encodes the cache and writes it atomically: marshal, write to a
// sibling temp file, then rename over the target path so partial writes can
// never corrupt the cache.
func (c *Cache) Save() error {
	dir := filepath.Dir(c.Path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling PR cache: %w", err)
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(c.Path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("creating cache temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing cache temp file: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("chmod cache temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing cache temp file: %w", err)
	}

	if err := os.Rename(tmpPath, c.Path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming cache temp file: %w", err)
	}
	return nil
}

// GetForQuery returns the cached entry for query and whether one was found.
func (c *Cache) GetForQuery(query string) (CacheEntry, bool) {
	if c.Entries == nil {
		return CacheEntry{}, false
	}
	entry, ok := c.Entries[query]
	return entry, ok
}

// SetForQuery inserts or replaces the entry for the given query. If adding a
// new query would exceed maxEntries, the oldest entry (by LastUpdated) is
// evicted first.
func (c *Cache) SetForQuery(query string, prs []github.PR, samlErrors []github.SAMLErrorEntry) {
	if c.Entries == nil {
		c.Entries = make(map[string]CacheEntry)
	}

	if _, exists := c.Entries[query]; !exists && len(c.Entries) >= maxEntries {
		var oldestKey string
		var oldestAt time.Time
		first := true
		for k, e := range c.Entries {
			if first || e.LastUpdated.Before(oldestAt) {
				oldestKey = k
				oldestAt = e.LastUpdated
				first = false
			}
		}
		delete(c.Entries, oldestKey)
	}

	c.Entries[query] = CacheEntry{
		PRs:         prs,
		Query:       query,
		LastUpdated: time.Now(),
		SAMLErrors:  samlErrors,
	}
}
