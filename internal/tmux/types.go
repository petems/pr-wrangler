package tmux

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// CommandRunner abstracts shell command execution for testability
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ExecRunner runs shell commands using the local process environment.
type ExecRunner struct{}

func (e *ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return out, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
	}
	return out, err
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
