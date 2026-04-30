package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// View represents a saved PR query view
type View struct {
	Name    string `yaml:"name"`
	Query   string `yaml:"query"`
	Default bool   `yaml:"default"`
}

// Config holds application configuration
type Config struct {
	Views              []View            `yaml:"views"`
	RepoBaseDir        string            `yaml:"repo_base_dir"`
	ServiceLabelPrefix string            `yaml:"service_label_prefix"`
	AgentCommands      map[string]string `yaml:"agent_commands"`
	OAuthClientID      string            `yaml:"oauth_client_id,omitempty"`

	// Path is the file path consulted during Load. Set even when the file
	// does not exist (in which case Loaded is false). Not serialized.
	Path   string `yaml:"-"`
	Loaded bool   `yaml:"-"`
}

// DefaultConfig returns a config with a single default view and generalized defaults
func DefaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		Views:              []View{{Name: "My PRs", Query: "author:@me is:open", Default: true}},
		RepoBaseDir:        filepath.Join(home, "projects"),
		ServiceLabelPrefix: "service:",
		AgentCommands: map[string]string{
			"fix-ci":            "claude --permission-mode acceptEdits 'The CI checks are failing on this PR: {{pr_url}} - Investigate the failing checks, identify the root cause, and fix the issues.'",
			"address-feedback":  "claude --permission-mode acceptEdits 'This PR has review feedback that needs to be addressed: {{pr_url}} - Read the review comments and make the requested changes.'",
			"resolve-conflicts": "claude --permission-mode acceptEdits 'This PR has merge conflicts: {{pr_url}} - Resolve the conflicts while preserving the intended changes.'",
			"review-comments":   "claude --permission-mode acceptEdits '/pr-wrangler-review-comments {{pr_url}}'",
			"followup":          "claude --permission-mode acceptEdits 'Continue working on this PR: {{pr_url}} - Review the current state, check for any issues, and make progress on remaining work.'",
		},
	}
}

// ConfigPath returns the path to the config file
func ConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("finding config dir: %w", err)
	}
	return filepath.Join(dir, "pr-wrangler", "config.yaml"), nil
}

// Load reads config from the OS-appropriate config dir
// (~/.config/pr-wrangler/config.yaml on Linux, ~/Library/Application
// Support/pr-wrangler/config.yaml on macOS). Returns DefaultConfig if the
// file does not exist.
func Load() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		cfg := DefaultConfig()
		return cfg, nil
	}
	return LoadFromPath(path)
}

// Save writes config to the given file path, creating directories as needed
func Save(cfg Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

// LoadFromPath reads config from the given file path.
// Returns DefaultConfig if the file does not exist.
func LoadFromPath(path string) (Config, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is user's own config file
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg := DefaultConfig()
			cfg.Path = path
			cfg.Loaded = false
			return cfg, nil
		}
		return Config{}, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
	}

	if len(cfg.Views) == 0 {
		cfg.Views = DefaultConfig().Views
	}
	if cfg.RepoBaseDir == "" {
		cfg.RepoBaseDir = DefaultConfig().RepoBaseDir
	}
	if cfg.ServiceLabelPrefix == "" {
		cfg.ServiceLabelPrefix = DefaultConfig().ServiceLabelPrefix
	}
	if cfg.AgentCommands == nil {
		cfg.AgentCommands = DefaultConfig().AgentCommands
	}

	cfg.Path = path
	cfg.Loaded = true
	return cfg, nil
}
