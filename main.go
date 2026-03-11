package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/petems/pr-wrangler/internal/config"
	"github.com/petems/pr-wrangler/internal/github"
	"github.com/petems/pr-wrangler/internal/session"
	"github.com/petems/pr-wrangler/internal/tmux"
	"github.com/petems/pr-wrangler/internal/tui"
)

func main() {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Initialize components
	runner := &github.ExecRunner{}
	ghClient := &github.GHClient{Runner: runner}

	homeDir, _ := os.UserHomeDir()
	sessionMgr := tmux.NewSessionManager(runner, homeDir, cfg.RepoBaseDir)

	historyPath, _ := sessionHistoryPath()
	sessionStore := session.NewStore(historyPath)

	// Run TUI
	m := tui.NewModel(ghClient, sessionMgr, sessionStore, cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}

func sessionHistoryPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("finding config dir: %w", err)
	}
	return filepath.Join(dir, "pr-wrangler", "history.json"), nil
}
