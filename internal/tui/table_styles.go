package tui

import (
	"charm.land/lipgloss/v2"

	"github.com/petems/pr-wrangler/internal/github"
)

// columnHeaderStyle returns the lipgloss.Style applied to the header cell of a
// given column index in the PR table.
func (m *Model) columnHeaderStyle(col int) lipgloss.Style {
	switch col {
	case 1:
		return m.styles.Repo.Bold(true)
	case 2:
		return m.styles.Number.Bold(true)
	case 3:
		return m.styles.TitleText.Bold(true)
	case 4, 5:
		return m.styles.Header
	case 6:
		return m.styles.Header
	case 7:
		return m.styles.Header
	default:
		return m.styles.Header
	}
}

// columnBodyStyle returns the lipgloss.Style applied to a body cell in the PR
// table for the given row and column index.
func (m *Model) columnBodyStyle(r PRRow, col int) lipgloss.Style {
	switch col {
	case 1:
		return m.styles.Repo
	case 2:
		return m.styles.Number
	case 3:
		return m.styles.TitleText
	case 4, 5:
		return m.styles.Help
	case 6:
		return m.statusStyle(r.Status)
	case 7:
		return m.actionStyle(r.Action)
	default:
		return lipgloss.NewStyle().Foreground(m.styles.TableText)
	}
}

// statusStyle maps a PR status to its semantic lipgloss.Style.
func (m *Model) statusStyle(status github.PRStatus) lipgloss.Style {
	switch status {
	case github.PRStatusApproved, github.PRStatusMerged:
		return m.styles.Success
	case github.PRStatusCIFailing, github.PRStatusDraftCIFailing:
		return m.styles.Error.Bold(true)
	case github.PRStatusChangesRequested, github.PRStatusReviewedWithComments:
		return m.styles.Review
	case github.PRStatusHasConflicts:
		return m.styles.Conflict
	case github.PRStatusWaitingForReviews, github.PRStatusOpen, github.PRStatusSAMLRequired:
		return m.styles.Info
	case github.PRStatusDraft:
		return m.styles.Draft
	default:
		return lipgloss.NewStyle().Foreground(m.styles.TableText)
	}
}

// actionStyle maps a PR action to its semantic lipgloss.Style.
func (m *Model) actionStyle(action github.Action) lipgloss.Style {
	switch action {
	case github.ActionMerge:
		return m.styles.Success
	case github.ActionFixCI:
		return m.styles.Error.Bold(true)
	case github.ActionAddressFeedback, github.ActionReviewComments:
		return m.styles.Review
	case github.ActionResolveConflicts:
		return m.styles.Conflict
	case github.ActionInvestigate, github.ActionAuthorizeSAML:
		return m.styles.Info.Bold(true)
	default:
		return m.styles.Help
	}
}
