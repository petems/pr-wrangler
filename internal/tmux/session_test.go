package tmux

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// mockRunner records calls and returns pre-configured responses.
type mockRunner struct {
	calls   [][]string // each call is [name, arg0, arg1, ...]
	outputs map[string]mockResult
}

type mockResult struct {
	out []byte
	err error
}

func newMockRunner() *mockRunner {
	return &mockRunner{outputs: make(map[string]mockResult)}
}

func (r *mockRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	call := append([]string{name}, args...)
	r.calls = append(r.calls, call)

	key := strings.Join(call, " ")
	if res, ok := r.outputs[key]; ok {
		return res.out, res.err
	}
	return nil, nil
}

func (r *mockRunner) set(cmd string, out string, err error) {
	r.outputs[cmd] = mockResult{out: []byte(out), err: err}
}

func (r *mockRunner) lastCall() []string {
	if len(r.calls) == 0 {
		return nil
	}
	return r.calls[len(r.calls)-1]
}

func TestSessionExists_True(t *testing.T) {
	runner := newMockRunner()
	mgr := NewSessionManager(runner, "/home/test", "/home/test/src")

	// has-session succeeds → session exists
	exists := mgr.SessionExists(context.Background(), "my-pr-42")
	if !exists {
		t.Error("expected SessionExists to return true when has-session succeeds")
	}
	if got := runner.lastCall(); len(got) < 4 || got[3] != "my-pr-42" {
		t.Errorf("unexpected args: %v", got)
	}
}

func TestSessionExists_False(t *testing.T) {
	runner := newMockRunner()
	runner.set("tmux has-session -t no-such", "", fmt.Errorf("no session"))
	mgr := NewSessionManager(runner, "/home/test", "/home/test/src")

	exists := mgr.SessionExists(context.Background(), "no-such")
	if exists {
		t.Error("expected SessionExists to return false when has-session fails")
	}
}

func TestWindowExists_Found(t *testing.T) {
	runner := newMockRunner()
	runner.set("tmux list-windows -t sess -F #{window_name}", "bash\nci-fix\nclaude\n", nil)
	mgr := NewSessionManager(runner, "/home/test", "/home/test/src")

	if !mgr.WindowExists(context.Background(), "sess", "ci-fix") {
		t.Error("expected WindowExists to find 'ci-fix'")
	}
}

func TestWindowExists_NotFound(t *testing.T) {
	runner := newMockRunner()
	runner.set("tmux list-windows -t sess -F #{window_name}", "bash\nclaude\n", nil)
	mgr := NewSessionManager(runner, "/home/test", "/home/test/src")

	if mgr.WindowExists(context.Background(), "sess", "ci-fix") {
		t.Error("expected WindowExists to return false for 'ci-fix'")
	}
}

func TestWindowExists_ErrorReturnsFalse(t *testing.T) {
	runner := newMockRunner()
	runner.set("tmux list-windows -t bad -F #{window_name}", "", fmt.Errorf("no session"))
	mgr := NewSessionManager(runner, "/home/test", "/home/test/src")

	if mgr.WindowExists(context.Background(), "bad", "anything") {
		t.Error("expected WindowExists to return false on error")
	}
}

func TestCreateSessionWithWindow(t *testing.T) {
	runner := newMockRunner()
	mgr := NewSessionManager(runner, "/home/test", "/home/test/src")

	sess := PRSession{SessionName: "fix-bug-42", WorkDir: "/home/test/src/myrepo"}
	err := mgr.CreateSessionWithWindow(context.Background(), sess, "ci-fix", "claude --permission-mode acceptEdits '/fix-ci'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	call := runner.lastCall()
	// Expect: tmux new-session -d -s fix-bug-42 -n ci-fix -c /home/test/src/myrepo <cmd>
	if len(call) < 9 {
		t.Fatalf("expected at least 9 args, got %d: %v", len(call), call)
	}
	if call[0] != "tmux" || call[1] != "new-session" {
		t.Errorf("expected tmux new-session, got %s %s", call[0], call[1])
	}
	if call[4] != "fix-bug-42" {
		t.Errorf("session name: got %s, want fix-bug-42", call[4])
	}
	if call[6] != "ci-fix" {
		t.Errorf("window name: got %s, want ci-fix", call[6])
	}
	if call[8] != "/home/test/src/myrepo" {
		t.Errorf("workdir: got %s, want /home/test/src/myrepo", call[8])
	}
}

func TestCreateSessionWithWindow_NoCmd(t *testing.T) {
	runner := newMockRunner()
	mgr := NewSessionManager(runner, "/home/test", "/home/test/src")

	sess := PRSession{SessionName: "test-1", WorkDir: "/tmp"}
	_ = mgr.CreateSessionWithWindow(context.Background(), sess, "shell", "")

	call := runner.lastCall()
	// Without shellCmd, should not append extra arg
	if len(call) != 9 {
		t.Errorf("expected 9 args (no cmd), got %d: %v", len(call), call)
	}
}

func TestCreateNamedWindow(t *testing.T) {
	runner := newMockRunner()
	mgr := NewSessionManager(runner, "/home/test", "/home/test/src")

	err := mgr.CreateNamedWindow(context.Background(), "my-sess", "feedback", "/home/test/src/repo", "claude '/feedback'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	call := runner.lastCall()
	if call[0] != "tmux" || call[1] != "new-window" {
		t.Errorf("expected tmux new-window, got %s %s", call[0], call[1])
	}
	if call[3] != "my-sess" {
		t.Errorf("session: got %s, want my-sess", call[3])
	}
	if call[5] != "feedback" {
		t.Errorf("window: got %s, want feedback", call[5])
	}
}

func TestSanitizeSessionName_EmptyTitleUsesFallback(t *testing.T) {
	got := SanitizeSessionName("!!!", 123)
	if got != "pr-123" {
		t.Fatalf("got %q, want %q", got, "pr-123")
	}
}

func TestSwitchToSession_InsideTmux(t *testing.T) {
	runner := newMockRunner()
	mgr := NewSessionManager(runner, "/home/test", "/home/test/src")

	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	err := mgr.SwitchToSession(context.Background(), "target-sess")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	call := runner.lastCall()
	if call[0] != "tmux" || call[1] != "switch-client" || call[3] != "target-sess" {
		t.Errorf("unexpected call: %v", call)
	}
}

func TestSwitchToSession_OutsideTmux(t *testing.T) {
	runner := newMockRunner()
	mgr := NewSessionManager(runner, "/home/test", "/home/test/src")

	t.Setenv("TMUX", "")

	err := mgr.SwitchToSession(context.Background(), "target-sess")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	call := runner.lastCall()
	if call[0] != "tmux" || call[1] != "attach-session" || call[3] != "target-sess" {
		t.Errorf("expected attach-session, got: %v", call)
	}
}

func TestSwitchToSession_Error(t *testing.T) {
	runner := newMockRunner()
	runner.set("tmux switch-client -t bad-sess", "", fmt.Errorf("no such session"))
	mgr := NewSessionManager(runner, "/home/test", "/home/test/src")

	t.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	err := mgr.SwitchToSession(context.Background(), "bad-sess")
	if err == nil {
		t.Error("expected error from SwitchToSession")
	}
}

func TestListTmuxSessions_EmptyOutput(t *testing.T) {
	runner := newMockRunner()
	runner.set("tmux list-sessions -F #{session_name}", "", nil)
	mgr := NewSessionManager(runner, "/home/test", "/home/test/src")

	sessions, err := mgr.ListTmuxSessions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("got %v, want empty list", sessions)
	}
}

func TestEnsureWorktree_ExistingWorktree(t *testing.T) {
	runner := newMockRunner()
	repoDir := "/home/test/src/myrepo"
	existingPath := "/home/test/src/myrepo-worktrees/feature-xyz"
	runner.set(
		"git -C /home/test/src/myrepo worktree list --porcelain",
		fmt.Sprintf("worktree %s\nbranch refs/heads/feature-xyz\n", existingPath),
		nil,
	)
	mgr := NewSessionManager(runner, "/home/test", "/home/test/src")

	got, err := mgr.EnsureWorktree(context.Background(), repoDir, "feature-xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != existingPath {
		t.Fatalf("path: got %q, want %q", got, existingPath)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected only worktree list call, got %d calls: %v", len(runner.calls), runner.calls)
	}
}

func TestEnsureWorktree_CreatesNew(t *testing.T) {
	runner := newMockRunner()
	repoDir := "/home/test/src/myrepo"
	worktreeDir := "/home/test/src/myrepo-worktrees/feature-xyz"
	runner.set("git -C /home/test/src/myrepo worktree list --porcelain", "", nil)
	runner.set("git -C /home/test/src/myrepo fetch origin feature-xyz", "", nil)
	runner.set("git -C /home/test/src/myrepo worktree add /home/test/src/myrepo-worktrees/feature-xyz feature-xyz", "", nil)
	mgr := NewSessionManager(runner, "/home/test", "/home/test/src")

	got, err := mgr.EnsureWorktree(context.Background(), repoDir, "feature-xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != worktreeDir {
		t.Fatalf("path: got %q, want %q", got, worktreeDir)
	}
	if len(runner.calls) != 3 {
		t.Fatalf("expected 3 calls, got %d: %v", len(runner.calls), runner.calls)
	}
}

func TestEnsureWorktree_FetchError(t *testing.T) {
	runner := newMockRunner()
	repoDir := "/home/test/src/myrepo"
	runner.set("git -C /home/test/src/myrepo worktree list --porcelain", "", nil)
	runner.set("git -C /home/test/src/myrepo fetch origin feature-xyz", "", fmt.Errorf("fetch failed"))
	mgr := NewSessionManager(runner, "/home/test", "/home/test/src")

	_, err := mgr.EnsureWorktree(context.Background(), repoDir, "feature-xyz")
	if err == nil {
		t.Fatal("expected error")
	}
	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d: %v", len(runner.calls), runner.calls)
	}
}

func TestWorktreeDir(t *testing.T) {
	mgr := NewSessionManager(newMockRunner(), "/home/test", "/home/test/src")

	got := mgr.WorktreeDir("myrepo", "feature-xyz")
	want := filepath.Join("/home/test/src", "myrepo-worktrees", "feature-xyz")
	if got != want {
		t.Fatalf("path: got %q, want %q", got, want)
	}
}
