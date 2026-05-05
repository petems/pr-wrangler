package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/petems/pr-wrangler/internal/config"
	"github.com/petems/pr-wrangler/internal/github"
	"github.com/petems/pr-wrangler/internal/tmux"
)

type acceptanceMockRunner struct {
	calls   [][]string
	outputs map[string]acceptanceMockResult
}

type acceptanceMockResult struct {
	out []byte
	err error
}

func newAcceptanceMockRunner() *acceptanceMockRunner {
	return &acceptanceMockRunner{outputs: make(map[string]acceptanceMockResult)}
}

func (r *acceptanceMockRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	call := append([]string{name}, args...)
	r.calls = append(r.calls, call)

	key := strings.Join(call, " ")
	if res, ok := r.outputs[key]; ok {
		return res.out, res.err
	}
	return nil, nil
}

func (r *acceptanceMockRunner) set(cmd string, out string, err error) {
	r.outputs[cmd] = acceptanceMockResult{out: []byte(out), err: err}
}

func newAcceptanceTestModel(t *testing.T, fetcher *MockPRFetcher, query string) Model {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.Views = []config.View{{Name: "acceptance", Query: query, Default: true}}
	cfg.RepoBaseDir = t.TempDir()

	runner := newAcceptanceMockRunner()
	runner.set("tmux list-sessions -F #{session_name}", "", nil)

	m := NewModel(fetcher, tmux.NewSessionManager(runner, t.TempDir(), cfg.RepoBaseDir), nil, cfg)
	m.width = 140
	m.height = 40
	return m
}

func driveAcceptanceModelReady(t *testing.T, m Model) Model {
	t.Helper()

	cmds := []tea.Cmd{m.Init()}
	for len(cmds) > 0 && (m.loading || m.progressCh != nil) {
		cmd := cmds[0]
		cmds = cmds[1:]
		if cmd == nil {
			continue
		}

		msg := cmd()
		switch msg := msg.(type) {
		case nil:
			continue
		case tea.BatchMsg:
			cmds = append(cmds, msg...)
			continue
		case spinner.TickMsg:
			continue
		}

		updated, next := m.Update(msg)
		var ok bool
		m, ok = updated.(Model)
		if !ok {
			t.Fatalf("updated model type: got %T", updated)
		}
		if next != nil {
			cmds = append(cmds, next)
		}
	}

	if m.loading {
		t.Fatalf("model still loading after driving lifecycle")
	}
	return m
}

func assertViewContains(t *testing.T, m Model, want string) {
	t.Helper()
	view := stripANSI(m.View())
	if !strings.Contains(view, want) {
		t.Fatalf("view missing %q:\n%s", want, view)
	}
}

func assertViewNotContains(t *testing.T, m Model, unwanted string) {
	t.Helper()
	view := stripANSI(m.View())
	if strings.Contains(view, unwanted) {
		t.Fatalf("view unexpectedly contains %q:\n%s", unwanted, view)
	}
}

func TestAcceptancePRListRendersTitlesStatusesAndActions(t *testing.T) {
	t.Parallel()

	fetcher := &MockPRFetcher{PRs: []github.PR{
		acceptanceOpenCIFailingPR(),
		acceptanceApprovedMergeablePR(),
		acceptanceChangesRequestedPR(),
		acceptanceDraftPR(),
		acceptanceConflictingPR(),
		acceptanceMergedPR(),
		acceptanceClosedPR(),
	}}
	m := driveAcceptanceModelReady(t, newAcceptanceTestModel(t, fetcher, "author:@me is:open"))

	assertViewContains(t, m, "Fix failing payment specs")
	assertViewContains(t, m, "CI failing")
	assertViewContains(t, m, "Fix CI")
	assertViewContains(t, m, "Add invoice export")
	assertViewContains(t, m, "Approved")
	assertViewContains(t, m, "Merge")
	assertViewContains(t, m, "Refactor webhook retries")
	assertViewContains(t, m, "Changes requested")
	assertViewContains(t, m, "Address feedback")
	assertViewContains(t, m, "Draft settlement importer")
	assertViewContains(t, m, "Draft")
	assertViewContains(t, m, "Resolve checkout conflicts")
	assertViewContains(t, m, "Has conflicts")
	assertViewContains(t, m, "Resolve conflicts")
	assertViewNotContains(t, m, "Merged cleanup")
	assertViewNotContains(t, m, "Closed experiment")
}

func TestAcceptanceUsesConfiguredViewQuery(t *testing.T) {
	t.Parallel()

	const query = "review-requested:@me is:open"
	fetcher := &MockPRFetcher{PRs: []github.PR{acceptanceApprovedMergeablePR()}}
	m := driveAcceptanceModelReady(t, newAcceptanceTestModel(t, fetcher, query))

	if len(fetcher.Queries) != 1 {
		t.Fatalf("queries: got %d, want 1", len(fetcher.Queries))
	}
	if fetcher.Queries[0] != query {
		t.Fatalf("query: got %q, want %q", fetcher.Queries[0], query)
	}
	assertViewContains(t, m, "Add invoice export")
}

func TestAcceptanceDisplaysPartialSAMLErrors(t *testing.T) {
	t.Parallel()

	fetcher := &MockPRFetcher{
		PRs:    []github.PR{acceptanceApprovedMergeablePR()},
		Errors: []github.SAMLErrorEntry{acceptanceSAMLErrorEntry(1, 222)},
	}
	m := driveAcceptanceModelReady(t, newAcceptanceTestModel(t, fetcher, "author:@me is:open"))

	assertViewContains(t, m, "Add invoice export")
	assertViewContains(t, m, "SAML Authorization Required")
	assertViewContains(t, m, "SAML Auth Required")
	assertViewContains(t, m, "Authorize SAML")
}

func TestAcceptanceDisplaysCompleteFetchFailure(t *testing.T) {
	t.Parallel()

	fetcher := &MockPRFetcher{Err: errors.New("github unavailable")}
	m := driveAcceptanceModelReady(t, newAcceptanceTestModel(t, fetcher, "author:@me is:open"))

	assertViewContains(t, m, "Error: github unavailable")
}

func TestAcceptanceLoadingStateRendersProgress(t *testing.T) {
	t.Parallel()

	m := newAcceptanceTestModel(t, &MockPRFetcher{}, "author:@me is:open")
	progressCh := make(chan tea.Msg)
	m.progressCh = progressCh
	m.progressDone = 1
	m.progressTotal = 3

	assertViewContains(t, m, "Fetching PR's")
	assertViewContains(t, m, "1/3 PRs")
	assertViewContains(t, m, "query: author:@me is:open is:pr")
}

func TestAcceptanceEmptyStateWhenNoPRsMatch(t *testing.T) {
	t.Parallel()

	m := driveAcceptanceModelReady(t, newAcceptanceTestModel(t, &MockPRFetcher{}, "author:@me is:open"))

	if len(m.rows) != 0 {
		t.Fatalf("rows: got %d, want 0", len(m.rows))
	}
	assertViewContains(t, m, "PR Wrangler")
	assertViewNotContains(t, m, "#101")
}
