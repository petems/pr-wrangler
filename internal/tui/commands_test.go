package tui

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/petems/pr-wrangler/internal/tmux"
)

// fakeRunner records every Run() call in order and returns pre-configured
// outputs/errors keyed by the joined command string. It mirrors the mock
// runner used in internal/tmux/session_test.go but lives locally so the
// tui package's tests stay self-contained.
type fakeRunner struct {
	calls   [][]string
	outputs map[string]fakeResult
}

type fakeResult struct {
	out []byte
	err error
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{outputs: make(map[string]fakeResult)}
}

func (r *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	call := append([]string{name}, args...)
	r.calls = append(r.calls, call)
	if res, ok := r.outputs[strings.Join(call, " ")]; ok {
		return res.out, res.err
	}
	return nil, nil
}

func (r *fakeRunner) set(cmd string, out string, err error) {
	r.outputs[cmd] = fakeResult{out: []byte(out), err: err}
}

func TestEnsureSessionCmd(t *testing.T) {
	const (
		sessionName = "fix-bug-42"
		workDir     = "/home/test/src/myrepo-worktrees/feature-xyz"
		windowName  = "ci-fix"
		shellCmd    = "claude --permission-mode acceptEdits '/fix-ci'"
	)
	sess := tmux.PRSession{
		SessionName: sessionName,
		WorkDir:     workDir,
		Branch:      "feature-xyz",
	}

	cases := []struct {
		name      string
		setup     func(*fakeRunner)
		wantCalls [][]string
	}{
		{
			name: "brand-new session seeds shell then action then selects action",
			setup: func(r *fakeRunner) {
				// has-session fails → session does not exist
				r.set("tmux has-session -t "+sessionName, "", fmt.Errorf("no session"))
			},
			wantCalls: [][]string{
				{"tmux", "has-session", "-t", sessionName},
				{"tmux", "new-session", "-d", "-s", sessionName, "-n", "shell", "-c", workDir},
				{"tmux", "new-window", "-t", sessionName, "-n", windowName, "-c", workDir, shellCmd},
				{"tmux", "select-window", "-t", sessionName + ":" + windowName},
			},
		},
		{
			name: "existing session, missing action window adds window only",
			setup: func(r *fakeRunner) {
				// has-session succeeds (default nil error)
				r.set(
					"tmux list-windows -t "+sessionName+" -F #{window_name}",
					"shell\nfeedback\n",
					nil,
				)
			},
			wantCalls: [][]string{
				{"tmux", "has-session", "-t", sessionName},
				{"tmux", "list-windows", "-t", sessionName, "-F", "#{window_name}"},
				{"tmux", "new-window", "-t", sessionName, "-n", windowName, "-c", workDir, shellCmd},
			},
		},
		{
			name: "existing session and window does nothing mutating",
			setup: func(r *fakeRunner) {
				r.set(
					"tmux list-windows -t "+sessionName+" -F #{window_name}",
					"shell\n"+windowName+"\n",
					nil,
				)
			},
			wantCalls: [][]string{
				{"tmux", "has-session", "-t", sessionName},
				{"tmux", "list-windows", "-t", sessionName, "-F", "#{window_name}"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := newFakeRunner()
			tc.setup(runner)
			mgr := tmux.NewSessionManager(runner, "/home/test", "/home/test/src")

			msg := ensureSessionCmd(mgr, sess, windowName, shellCmd)()

			ready, ok := msg.(sessionReadyMsg)
			if !ok {
				t.Fatalf("expected sessionReadyMsg, got %T: %#v", msg, msg)
			}
			if ready.sessionName != sessionName || ready.windowName != windowName {
				t.Errorf("sessionReadyMsg: got %+v, want {sessionName:%q windowName:%q}",
					ready, sessionName, windowName)
			}

			if !reflect.DeepEqual(runner.calls, tc.wantCalls) {
				t.Errorf("call sequence mismatch:\n  got:  %v\n  want: %v", runner.calls, tc.wantCalls)
			}
		})
	}
}
