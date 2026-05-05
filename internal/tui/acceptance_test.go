//go:build acceptance

package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/petems/pr-wrangler/internal/config"
	"github.com/petems/pr-wrangler/internal/github"
	"github.com/petems/pr-wrangler/internal/session"
	"github.com/petems/pr-wrangler/internal/tmux"
)

type acceptanceMockRunner struct{}

func (r *acceptanceMockRunner) Run(_ context.Context, _ string, _ ...string) ([]byte, error) {
	return nil, nil
}

func newAcceptanceTestModel(t *testing.T, fetcher *MockPRFetcher, cfg config.Config) Model {
	t.Helper()
	if cfg.AgentCommands == nil {
		cfg.AgentCommands = config.DefaultConfig().AgentCommands
	}
	if cfg.ColorScheme == "" {
		cfg.ColorScheme = config.DefaultConfig().ColorScheme
	}
	mgr := tmux.NewSessionManager(&acceptanceMockRunner{}, t.TempDir(), t.TempDir())
	store := session.NewStore(t.TempDir() + "/sessions.json")
	m := NewModel(fetcher, mgr, store, cfg)
	m.width = 160
	m.height = 40
	m.table = m.rebuildTable()
	return m
}

func loadAcceptanceModel(t *testing.T, m Model) Model {
	t.Helper()
	cmd := m.refreshCmd()
	msg := cmd()
	started, ok := msg.(prsFetchStartedMsg)
	if !ok {
		t.Fatalf("refreshCmd returned %T, want prsFetchStartedMsg", msg)
	}
	updated, cmd := m.Update(started)
	m = updated.(Model)
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			t.Fatal("fetch channel closed before prsLoadedMsg")
		}
		updated, cmd = m.Update(msg)
		m = updated.(Model)
		if _, done := msg.(prsLoadedMsg); done {
			break
		}
	}
	return m
}

func assertViewContains(t *testing.T, m Model, want string) {
	t.Helper()
	if !strings.Contains(stripANSI(m.View()), want) {
		t.Fatalf("view missing %q:\n%s", want, stripANSI(m.View()))
	}
}

func assertViewNotContains(t *testing.T, m Model, unwanted string) {
	t.Helper()
	if strings.Contains(stripANSI(m.View()), unwanted) {
		t.Fatalf("view unexpectedly contained %q:\n%s", unwanted, stripANSI(m.View()))
	}
}

func TestAcceptancePRListRendersStatusesAndActions(t *testing.T) {
	fetcher := &MockPRFetcher{PRs: []github.PR{
		acceptancePROpenCIFailing(),
		acceptancePRApprovedMergeable(),
		acceptancePRChangesRequested(),
		acceptancePRDraft(),
		acceptancePRConflicting(),
		acceptancePRMerged(),
		acceptancePRClosed(),
	}}
	m := loadAcceptanceModel(t, newAcceptanceTestModel(t, fetcher, config.DefaultConfig()))

	assertViewContains(t, m, "Fix payment retry")
	assertViewContains(t, m, "CI failing")
	assertViewContains(t, m, "Fix CI")
	assertViewContains(t, m, "Add billing export")
	assertViewContains(t, m, "Approved")
	assertViewContains(t, m, "Merge")
	assertViewContains(t, m, "Tighten auth checks")
	assertViewContains(t, m, "Changes requested")
	assertViewContains(t, m, "Address feedback")
	assertViewContains(t, m, "Draft dashboard refresh")
	assertViewContains(t, m, "Draft")
	assertViewContains(t, m, "Resolve branch drift")
	assertViewContains(t, m, "Has conflicts")
	assertViewContains(t, m, "Resolve conflicts")
	assertViewNotContains(t, m, "Merged cleanup")
	assertViewNotContains(t, m, "Closed experiment")
}

func TestAcceptanceConfiguredQueryIsUsedForFiltering(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Views = []config.View{{Name: "Review", Query: "review-requested:@me is:open", Default: true}}
	fetcher := &MockPRFetcher{PRs: []github.PR{acceptancePRApprovedMergeable()}}

	m := loadAcceptanceModel(t, newAcceptanceTestModel(t, fetcher, cfg))

	if len(fetcher.Queries) != 1 || fetcher.Queries[0] != "review-requested:@me is:open" {
		t.Fatalf("fetch query = %v, want configured query", fetcher.Queries)
	}
	assertViewContains(t, m, "[query: review-requested:@me is:open]")
	assertViewContains(t, m, "Add billing export")
}

func TestAcceptancePartialSAMLErrorRendersPlaceholder(t *testing.T) {
	fetcher := &MockPRFetcher{
		PRs:    []github.PR{acceptancePROpenCIFailing()},
		Errors: []github.SAMLErrorEntry{acceptanceSAMLError(1, "example/private", 201)},
	}
	m := loadAcceptanceModel(t, newAcceptanceTestModel(t, fetcher, config.DefaultConfig()))

	assertViewContains(t, m, "Fix payment retry")
	assertViewContains(t, m, "SAML Authorization Required")
	assertViewContains(t, m, "SAML Auth Required")
	assertViewContains(t, m, "Authorize SAML")
}

func TestAcceptanceLoadingStateRendersProgress(t *testing.T) {
	m := newAcceptanceTestModel(t, &MockPRFetcher{}, config.DefaultConfig())
	started := prsFetchStartedMsg{progressCh: make(chan tea.Msg)}
	updated, _ := m.Update(started)
	m = updated.(Model)
	assertViewContains(t, m, "Searching for PRs...")

	updated, _ = m.Update(prsProgressMsg{progressCh: started.progressCh, done: 2, total: 4})
	m = updated.(Model)
	assertViewContains(t, m, "2/4 PRs")
}

func TestAcceptanceEmptyStateWhenNoPRsMatch(t *testing.T) {
	m := loadAcceptanceModel(t, newAcceptanceTestModel(t, &MockPRFetcher{}, config.DefaultConfig()))

	assertViewContains(t, m, "No PRs match the current query.")
}

func TestAcceptanceCompleteFailureShowsError(t *testing.T) {
	fetcher := &MockPRFetcher{Err: errors.New("github unavailable")}
	m := loadAcceptanceModel(t, newAcceptanceTestModel(t, fetcher, config.DefaultConfig()))

	assertViewContains(t, m, "Error: github unavailable")
}
