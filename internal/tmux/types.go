package tmux

import (
	"context"
)

// CommandRunner abstracts shell command execution for testability
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// PRSession represents a persistent tmux session for a PR or local work
type PRSession struct {
	SessionName string // sanitized PR title + number
	PRNumber    int
	PRTitle     string // original title for display
	WorkDir     string // chosen worktree directory
	Branch      string // PR's HeadRefName
	PRURL       string // for reference
}

// Worktree represents a git worktree
type Worktree struct {
	Path   string
	Branch string
}

// RepoInfo holds information about a discovered repository
type RepoInfo struct {
	Repo         string
	Branch       string
	WorktreePath string
}
