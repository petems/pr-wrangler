package tui

import (
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/petems/pr-wrangler/internal/github"
)

// styleEqual returns true when two lipgloss.Styles render an identical sample
// string. This is the most reliable identity check for v2 styles since the
// struct contains unexported fields.
func styleEqual(a, b lipgloss.Style) bool {
	const sample = "sample"
	return a.Render(sample) == b.Render(sample)
}

func TestStatusStyle(t *testing.T) {
	m := &Model{styles: NewStyles("default")}

	cases := []struct {
		name   string
		status github.PRStatus
		want   lipgloss.Style
	}{
		{"approved", github.PRStatusApproved, m.styles.Success},
		{"merged", github.PRStatusMerged, m.styles.Success},
		{"ci failing", github.PRStatusCIFailing, m.styles.Error.Bold(true)},
		{"draft ci failing", github.PRStatusDraftCIFailing, m.styles.Error.Bold(true)},
		{"changes requested", github.PRStatusChangesRequested, m.styles.Review},
		{"reviewed with comments", github.PRStatusReviewedWithComments, m.styles.Review},
		{"has conflicts", github.PRStatusHasConflicts, m.styles.Conflict},
		{"waiting for reviews", github.PRStatusWaitingForReviews, m.styles.Info},
		{"open", github.PRStatusOpen, m.styles.Info},
		{"saml required", github.PRStatusSAMLRequired, m.styles.Info},
		{"draft", github.PRStatusDraft, m.styles.Draft},
		{"unknown default", github.PRStatus("not-a-real-status"), lipgloss.NewStyle().Foreground(m.styles.TableText)},
		{"zero value default", github.PRStatus(""), lipgloss.NewStyle().Foreground(m.styles.TableText)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := m.statusStyle(tc.status)
			if !styleEqual(got, tc.want) {
				t.Errorf("statusStyle(%q): rendered output differs from expected style", tc.status)
			}
		})
	}
}

func TestActionStyle(t *testing.T) {
	m := &Model{styles: NewStyles("default")}

	cases := []struct {
		name   string
		action github.Action
		want   lipgloss.Style
	}{
		{"merge", github.ActionMerge, m.styles.Success},
		{"fix ci", github.ActionFixCI, m.styles.Error.Bold(true)},
		{"address feedback", github.ActionAddressFeedback, m.styles.Review},
		{"review comments", github.ActionReviewComments, m.styles.Review},
		{"resolve conflicts", github.ActionResolveConflicts, m.styles.Conflict},
		{"investigate", github.ActionInvestigate, m.styles.Info.Bold(true)},
		{"authorize saml", github.ActionAuthorizeSAML, m.styles.Info.Bold(true)},
		{"action none default", github.ActionNone, m.styles.Help},
		{"unknown default", github.Action("not-a-real-action"), m.styles.Help},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := m.actionStyle(tc.action)
			if !styleEqual(got, tc.want) {
				t.Errorf("actionStyle(%q): rendered output differs from expected style", tc.action)
			}
		})
	}
}
