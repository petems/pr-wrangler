package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/petems/pr-wrangler/internal/cache"
	"github.com/petems/pr-wrangler/internal/config"
	"github.com/petems/pr-wrangler/internal/github"
)

func TestParseTUIOptions(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantNoCache bool
		wantErr     bool
	}{
		{
			name: "empty",
		},
		{
			name:        "no cache",
			args:        []string{"--no-cache"},
			wantNoCache: true,
		},
		{
			name:    "unknown flag",
			args:    []string{"--wat"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTUIOptions(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTUIOptions: %v", err)
			}
			if got.noCache != tt.wantNoCache {
				t.Fatalf("noCache = %v, want %v", got.noCache, tt.wantNoCache)
			}
		})
	}
}

func TestCacheDisabled(t *testing.T) {
	tests := []struct {
		name        string
		opts        tuiOptions
		cfg         config.Config
		wantDisable bool
	}{
		{
			name:        "enabled by config",
			cfg:         config.Config{CacheEnabled: true},
			wantDisable: false,
		},
		{
			name:        "disabled by config",
			cfg:         config.Config{CacheEnabled: false},
			wantDisable: true,
		},
		{
			name:        "flag overrides enabled config",
			opts:        tuiOptions{noCache: true},
			cfg:         config.Config{CacheEnabled: true},
			wantDisable: true,
		},
		{
			name:        "flag preserves disabled config",
			opts:        tuiOptions{noCache: true},
			cfg:         config.Config{CacheEnabled: false},
			wantDisable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cacheDisabled(tt.opts, tt.cfg); got != tt.wantDisable {
				t.Fatalf("cacheDisabled = %v, want %v", got, tt.wantDisable)
			}
		})
	}
}

func TestClearCacheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pr-cache.json")
	if err := os.WriteFile(path, []byte(`{"entries":{}}`), 0o600); err != nil {
		t.Fatalf("writing cache: %v", err)
	}

	if err := clearCacheFile(path); err != nil {
		t.Fatalf("clearCacheFile existing file: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("cache file still exists after clear: %v", err)
	}

	if err := clearCacheFile(path); err != nil {
		t.Fatalf("clearCacheFile missing file should be idempotent: %v", err)
	}
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	original := os.Stdout
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	os.Stdout = writeEnd

	fnErr := fn()
	if err := writeEnd.Close(); err != nil {
		t.Fatalf("closing pipe writer: %v", err)
	}
	os.Stdout = original

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(readEnd); err != nil {
		t.Fatalf("reading stdout: %v", err)
	}
	if err := readEnd.Close(); err != nil {
		t.Fatalf("closing pipe reader: %v", err)
	}
	return buf.String(), fnErr
}

func TestPrintCacheStatusMissingOrEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pr-cache.json")

	output, err := captureStdout(t, func() error {
		return printCacheStatus(path)
	})
	if err != nil {
		t.Fatalf("printCacheStatus missing file: %v", err)
	}
	if !strings.Contains(output, "PR cache path: "+path) {
		t.Fatalf("status output missing path:\n%s", output)
	}
	if !strings.Contains(output, "No cached PR queries.") {
		t.Fatalf("status output missing empty message:\n%s", output)
	}
}

func TestPrintCacheStatusListsEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pr-cache.json")
	prCache := cache.NewCache(path)
	prCache.Entries["z-query"] = cache.CacheEntry{
		Query:       "z-query",
		PRs:         []github.PR{{Number: 1}, {Number: 2}},
		SAMLErrors:  []cache.CachedSAMLErrorEntry{{Index: 1}},
		LastUpdated: time.Date(2026, 5, 15, 20, 55, 0, 0, time.UTC),
	}
	prCache.Entries["a-query"] = cache.CacheEntry{
		Query:       "a-query",
		PRs:         []github.PR{{Number: 3}},
		LastUpdated: time.Time{},
	}
	if err := prCache.Save(); err != nil {
		t.Fatalf("saving cache: %v", err)
	}

	output, err := captureStdout(t, func() error {
		return printCacheStatus(path)
	})
	if err != nil {
		t.Fatalf("printCacheStatus: %v", err)
	}

	for _, want := range []string{
		"Cached PR queries: 2",
		"- a-query",
		"  PRs: 1",
		"  SAML errors: 0",
		"  Last updated: unknown",
		"- z-query",
		"  PRs: 2",
		"  SAML errors: 1",
		"  Last updated: 2026-05-15T20:55:00Z",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("status output missing %q:\n%s", want, output)
		}
	}
	if strings.Index(output, "- a-query") > strings.Index(output, "- z-query") {
		t.Fatalf("cache entries should be sorted by query:\n%s", output)
	}
}
