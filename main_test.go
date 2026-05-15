package main

import (
	"testing"

	"github.com/petems/pr-wrangler/internal/config"
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
