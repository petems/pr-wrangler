package tmux

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	reNonAlphanumHyphen = regexp.MustCompile(`[^a-z0-9-]`)
	reMultiHyphen       = regexp.MustCompile(`-+`)
)

type SessionManager struct {
	Runner  CommandRunner
	HomeDir string
	BaseDir string // from config.RepoBaseDir
}

func NewSessionManager(runner CommandRunner, homeDir, baseDir string) *SessionManager {
	return &SessionManager{
		Runner:  runner,
		HomeDir: homeDir,
		BaseDir: baseDir,
	}
}

// RepoDir returns the path to a repo clone
func (m *SessionManager) RepoDir(repoName string) string {
	return filepath.Join(m.BaseDir, repoName)
}

// WorktreeDir returns the standardized path to a repo worktree checkout.
func (m *SessionManager) WorktreeDir(repoName, branch string) string {
	return filepath.Join(m.BaseDir, repoName+"-worktrees", branch)
}

// ListPRSessions returns all active tmux sessions that look like PR workspaces
func (m *SessionManager) ListPRSessions(ctx context.Context) (map[int]PRSession, error) {
	out, err := m.Runner.Run(ctx, "tmux", "list-sessions", "-F", "#{session_name}")
	if err != nil {
		// tmux returns error if no sessions exist
		return make(map[int]PRSession), nil
	}

	sessions := make(map[int]PRSession)
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		// Session names like "title-123"
		if prNum := extractPRNumber(line); prNum > 0 {
			sessions[prNum] = PRSession{SessionName: line, PRNumber: prNum}
		}
	}
	return sessions, nil
}

func extractPRNumber(sessionName string) int {
	parts := strings.Split(sessionName, "-")
	if len(parts) < 2 {
		return 0
	}
	var num int
	_, err := fmt.Sscanf(parts[len(parts)-1], "%d", &num)
	if err != nil {
		return 0
	}
	return num
}

// SanitizeSessionName creates a tmux-safe session name from a PR title and number
func SanitizeSessionName(title string, number int) string {
	s := strings.ToLower(title)
	s = strings.NewReplacer(".", "-", ":", "-", "/", "-", "\\", "-").Replace(s)
	s = reNonAlphanumHyphen.ReplaceAllString(s, "-")
	s = reMultiHyphen.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")

	if len(s) > 30 {
		s = s[:30]
	}
	s = strings.TrimRight(s, "-")
	if s == "" {
		s = "pr"
	}

	if number > 0 {
		return fmt.Sprintf("%s-%d", s, number)
	}
	return s
}

func (m *SessionManager) ListWorktrees(ctx context.Context, repoDir string) ([]Worktree, error) {
	out, err := m.Runner.Run(ctx, "git", "-C", repoDir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	var worktrees []Worktree
	var current Worktree
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			current.Path = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "branch ") {
			current.Branch = strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(current.Branch, "refs/heads/")
			worktrees = append(worktrees, current)
			current = Worktree{}
		} else if line == "" && current.Path != "" {
			// Detached HEAD or bare worktree
			worktrees = append(worktrees, current)
			current = Worktree{}
		}
	}
	return worktrees, nil
}

func (m *SessionManager) CreateWorktree(ctx context.Context, repoDir, path, branch string) error {
	_, err := m.Runner.Run(ctx, "git", "-C", repoDir, "worktree", "add", "-b", branch, path)
	return err
}

func (m *SessionManager) EnsureWorktree(ctx context.Context, repoDir, branch string) (string, error) {
	worktrees, err := m.ListWorktrees(ctx, repoDir)
	if err != nil {
		return "", err
	}

	for _, worktree := range worktrees {
		if worktree.Branch == branch {
			return worktree.Path, nil
		}
	}

	repoName := filepath.Base(repoDir)
	worktreeDir := m.WorktreeDir(repoName, branch)

	if _, err := m.Runner.Run(ctx, "git", "-C", repoDir, "fetch", "origin", branch); err != nil {
		return "", err
	}
	if _, err := m.Runner.Run(ctx, "git", "-C", repoDir, "worktree", "add", worktreeDir, branch); err != nil {
		return "", err
	}

	return worktreeDir, nil
}

func (m *SessionManager) ListRepos(ctx context.Context, baseDir string) ([]string, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, err
	}

	var repos []string
	for _, entry := range entries {
		if entry.IsDir() {
			// Check if it's a git repo
			if _, err := os.Stat(filepath.Join(baseDir, entry.Name(), ".git")); err == nil {
				repos = append(repos, entry.Name())
			}
		}
	}
	return repos, nil
}

func (m *SessionManager) ListTmuxSessions(ctx context.Context) ([]string, error) {
	out, err := m.Runner.Run(ctx, "tmux", "list-sessions", "-F", "#{session_name}")
	if err != nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return []string{}, nil
	}
	return strings.Split(trimmed, "\n"), nil
}

// SessionExists checks whether a tmux session with the given name exists.
func (m *SessionManager) SessionExists(ctx context.Context, name string) bool {
	_, err := m.Runner.Run(ctx, "tmux", "has-session", "-t", name)
	return err == nil
}

// WindowExists checks whether a named window exists in the given tmux session.
func (m *SessionManager) WindowExists(ctx context.Context, sessionName, windowName string) bool {
	out, err := m.Runner.Run(ctx, "tmux", "list-windows", "-t", sessionName, "-F", "#{window_name}")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == windowName {
			return true
		}
	}
	return false
}

// CreateSessionWithWindow creates a new tmux session with an initial named window,
// setting the working directory and running the given shell command.
func (m *SessionManager) CreateSessionWithWindow(ctx context.Context, sess PRSession, windowName, shellCmd string) error {
	args := []string{"new-session", "-d", "-s", sess.SessionName, "-n", windowName, "-c", sess.WorkDir}
	if shellCmd != "" {
		args = append(args, shellCmd)
	}
	_, err := m.Runner.Run(ctx, "tmux", args...)
	return err
}

// CreateNamedWindow adds a new named window to an existing tmux session.
func (m *SessionManager) CreateNamedWindow(ctx context.Context, sessionName, windowName, workDir, shellCmd string) error {
	args := []string{"new-window", "-t", sessionName, "-n", windowName, "-c", workDir}
	if shellCmd != "" {
		args = append(args, shellCmd)
	}
	_, err := m.Runner.Run(ctx, "tmux", args...)
	return err
}

// InsideTmux reports whether the current process is running inside a tmux session.
func (m *SessionManager) InsideTmux() bool {
	return os.Getenv("TMUX") != ""
}

// SwitchToSession switches to the given tmux session. If already inside tmux
// it uses switch-client; otherwise it attaches to the session.
func (m *SessionManager) SwitchToSession(ctx context.Context, sessionName string) error {
	if m.InsideTmux() {
		_, err := m.Runner.Run(ctx, "tmux", "switch-client", "-t", sessionName)
		return err
	}
	_, err := m.Runner.Run(ctx, "tmux", "attach-session", "-t", sessionName)
	return err
}
