package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigEnablesCache(t *testing.T) {
	if !DefaultConfig().CacheEnabled {
		t.Fatal("default config should enable cache")
	}
}

func TestLoadFromPathCacheEnabledDefaultAndExplicitFalse(t *testing.T) {
	tests := []struct {
		name      string
		contents  string
		wantCache bool
	}{
		{
			name:      "missing cache_enabled defaults true",
			contents:  "repo_base_dir: /tmp/projects\n",
			wantCache: true,
		},
		{
			name:      "explicit true",
			contents:  "cache_enabled: true\n",
			wantCache: true,
		},
		{
			name:      "explicit false",
			contents:  "cache_enabled: false\n",
			wantCache: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.contents), 0o600); err != nil {
				t.Fatalf("writing config: %v", err)
			}

			cfg, err := LoadFromPath(path)
			if err != nil {
				t.Fatalf("LoadFromPath: %v", err)
			}
			if cfg.CacheEnabled != tt.wantCache {
				t.Fatalf("CacheEnabled = %v, want %v", cfg.CacheEnabled, tt.wantCache)
			}
		})
	}
}

func TestLoadFromPathMissingFileEnablesCache(t *testing.T) {
	cfg, err := LoadFromPath(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("LoadFromPath: %v", err)
	}
	if !cfg.CacheEnabled {
		t.Fatal("missing config should use default cache_enabled=true")
	}
}
