package tui

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/petems/pr-wrangler/internal/github"
	"github.com/petems/pr-wrangler/internal/tmux"
)

type prsLoadedMsg struct {
	prs []github.PR
	err error
}

type sessionsDiscoveredMsg struct {
	sessions map[int]tmux.PRSession
}

type sessionCreatedMsg struct {
	sessionName string
	windowName  string
}

type sessionErrorMsg struct {
	err error
}

type worktreeReadyMsg struct {
	sess         tmux.PRSession
	windowName   string
	claudeCmd    string
	warning      string
	warningError error
}

// sessionReadyMsg is sent after the tmux session/window has been created,
// signaling that we should now switch to it.
type sessionReadyMsg struct {
	sessionName string
	windowName  string
}

func fetchPRsCmd(ghClient *github.GHClient, query string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		prs, err := ghClient.FetchPRs(ctx, query)
		return prsLoadedMsg{prs: prs, err: err}
	}
}

func discoverSessionsCmd(mgr *tmux.SessionManager) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		sessions, err := mgr.ListPRSessions(ctx)
		if err != nil {
			return sessionsDiscoveredMsg{sessions: make(map[int]tmux.PRSession)}
		}
		return sessionsDiscoveredMsg{sessions: sessions}
	}
}

func ensureWorktreeCmd(mgr *tmux.SessionManager, sess tmux.PRSession, repoDir, windowName, shellCmd string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		workDir, err := mgr.EnsureWorktree(ctx, repoDir, sess.Branch)
		if err != nil {
			sess.WorkDir = repoDir
			return worktreeReadyMsg{
				sess:         sess,
				windowName:   windowName,
				claudeCmd:    shellCmd,
				warning:      fmt.Sprintf("Worktree setup failed for %q, using base repo dir", sess.Branch),
				warningError: err,
			}
		}

		sess.WorkDir = workDir
		return worktreeReadyMsg{
			sess:       sess,
			windowName: windowName,
			claudeCmd:  shellCmd,
		}
	}
}

// ensureSessionCmd creates the tmux session/window if needed, then sends
// sessionReadyMsg so Update() can call switchClientCmd with tea.ExecProcess.
func ensureSessionCmd(mgr *tmux.SessionManager, sess tmux.PRSession, windowName, shellCmd string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if !mgr.SessionExists(ctx, sess.SessionName) {
			if err := mgr.CreateSessionWithWindow(ctx, sess, windowName, shellCmd); err != nil {
				return sessionErrorMsg{err: fmt.Errorf("creating session %q: %w", sess.SessionName, err)}
			}
		} else if !mgr.WindowExists(ctx, sess.SessionName, windowName) {
			if err := mgr.CreateNamedWindow(ctx, sess.SessionName, windowName, sess.WorkDir, shellCmd); err != nil {
				return sessionErrorMsg{err: fmt.Errorf("creating window %q: %w", windowName, err)}
			}
		}

		return sessionReadyMsg{sessionName: sess.SessionName, windowName: windowName}
	}
}

// switchClientCmd uses tea.ExecProcess to suspend Bubble Tea and run
// tmux switch-client (or attach-session), giving it proper terminal access.
func switchClientCmd(sessionName string, insideTmux bool) tea.Cmd {
	var c *exec.Cmd
	if insideTmux {
		c = exec.Command("tmux", "switch-client", "-t", sessionName)
	} else {
		c = exec.Command("tmux", "attach-session", "-t", sessionName)
	}
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return sessionErrorMsg{err: fmt.Errorf("switching to session %q: %w", sessionName, err)}
		}
		return sessionCreatedMsg{sessionName: sessionName}
	})
}
