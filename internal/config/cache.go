package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// CachePath returns the path to the on-disk PR cache file. Uses the same
// OS-appropriate config directory as ConfigPath and SessionsPath
// (~/.config/pr-wrangler/pr-cache.json on Linux, ~/Library/Application
// Support/pr-wrangler/pr-cache.json on macOS).
func CachePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("finding config dir: %w", err)
	}
	return filepath.Join(dir, "pr-wrangler", "pr-cache.json"), nil
}
