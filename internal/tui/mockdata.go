package tui

import (
	"time"

	"github.com/petems/pr-wrangler/internal/github"
	"github.com/petems/pr-wrangler/internal/tmux"
)

// MockPRs returns a fixed set of PRs covering a range of states so the demo
// view exercises every status/action branch the TUI can render.
func MockPRs() []github.PR {
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	mergedAt := now.Add(-24 * time.Hour)

	return []github.PR{
		{
			Number:            101,
			Title:             "Add OAuth device flow for GitHub authentication",
			URL:               "https://github.com/petems/pr-wrangler/pull/101",
			RepoNameWithOwner: "petems/pr-wrangler",
			Author:            "petems",
			HeadRefName:       "feat/oauth-device-flow",
			HeadCommitOID:     "abc1234567890abcdef1234567890abcdef12345",
			State:             github.PRStateOpen,
			Mergeable:         string(github.MergeableMergeable),
			CreatedAt:         now.Add(-72 * time.Hour),
			UpdatedAt:         now.Add(-2 * time.Hour),
			ReviewDecision:    github.ReviewDecisionApproved,
			LatestCheckState:  string(github.CIConclusionSuccess),
			StatusChecks: []github.StatusCheck{
				{Name: "test", Conclusion: github.CIConclusionSuccess, Status: "COMPLETED"},
				{Name: "lint", Conclusion: github.CIConclusionSuccess, Status: "COMPLETED"},
			},
			Reviews: []github.Review{
				{Author: "octocat", State: "APPROVED"},
			},
		},
		{
			Number:            102,
			Title:             "Migrate TUI to Charm v2 to fix OSC 8 hyperlink truncation",
			URL:               "https://github.com/petems/pr-wrangler/pull/102",
			RepoNameWithOwner: "petems/pr-wrangler",
			Author:            "petems",
			HeadRefName:       "fix/charm-v2-migration",
			State:             github.PRStateOpen,
			Mergeable:         string(github.MergeableMergeable),
			CreatedAt:         now.Add(-48 * time.Hour),
			UpdatedAt:         now.Add(-30 * time.Minute),
			ReviewDecision:    github.ReviewDecisionReviewRequired,
			LatestCheckState:  string(github.CIConclusionFailure),
			StatusChecks: []github.StatusCheck{
				{Name: "test", Conclusion: github.CIConclusionFailure, Status: "COMPLETED"},
				{Name: "lint", Conclusion: github.CIConclusionSuccess, Status: "COMPLETED"},
			},
		},
		{
			Number:            103,
			Title:             "Refactor session manager to support multi-window layouts",
			URL:               "https://github.com/petems/pr-wrangler/pull/103",
			RepoNameWithOwner: "petems/pr-wrangler",
			Author:            "contributor1",
			HeadRefName:       "refactor/session-manager",
			State:             github.PRStateOpen,
			Mergeable:         string(github.MergeableMergeable),
			CreatedAt:         now.Add(-96 * time.Hour),
			UpdatedAt:         now.Add(-1 * time.Hour),
			ReviewDecision:    github.ReviewDecisionChangesRequested,
			LatestCheckState:  string(github.CIConclusionSuccess),
			UnresolvedThreads: 3,
			StatusChecks: []github.StatusCheck{
				{Name: "test", Conclusion: github.CIConclusionSuccess, Status: "COMPLETED"},
			},
			Reviews: []github.Review{
				{Author: "petems", State: "CHANGES_REQUESTED"},
			},
		},
		{
			Number:            104,
			Title:             "Add real-time colour scheme switching with theme picker overlay",
			URL:               "https://github.com/petems/pr-wrangler/pull/104",
			RepoNameWithOwner: "petems/pr-wrangler",
			Author:            "contributor2",
			HeadRefName:       "feat/theme-picker",
			State:             github.PRStateOpen,
			Mergeable:         string(github.MergeableConflicting),
			CreatedAt:         now.Add(-120 * time.Hour),
			UpdatedAt:         now.Add(-6 * time.Hour),
			ReviewDecision:    github.ReviewDecisionReviewRequired,
			LatestCheckState:  string(github.CIConclusionSuccess),
		},
		{
			Number:            105,
			Title:             "WIP: experimental keybinding remapper",
			URL:               "https://github.com/petems/pr-wrangler/pull/105",
			RepoNameWithOwner: "petems/pr-wrangler",
			Author:            "petems",
			HeadRefName:       "wip/keybinding-remap",
			State:             github.PRStateOpen,
			IsDraft:           true,
			Mergeable:         string(github.MergeableMergeable),
			CreatedAt:         now.Add(-12 * time.Hour),
			UpdatedAt:         now.Add(-12 * time.Hour),
		},
		{
			Number:            106,
			Title:             "WIP: investigate flaky tmux integration tests",
			URL:               "https://github.com/petems/pr-wrangler/pull/106",
			RepoNameWithOwner: "petems/pr-wrangler",
			Author:            "petems",
			HeadRefName:       "wip/tmux-flake",
			State:             github.PRStateOpen,
			IsDraft:           true,
			Mergeable:         string(github.MergeableMergeable),
			CreatedAt:         now.Add(-18 * time.Hour),
			UpdatedAt:         now.Add(-18 * time.Hour),
			LatestCheckState:  string(github.CIConclusionFailure),
			StatusChecks: []github.StatusCheck{
				{Name: "integration-test", Conclusion: github.CIConclusionFailure, Status: "COMPLETED"},
			},
		},
		{
			Number:            107,
			Title:             "Document configuration loading order and defaults",
			URL:               "https://github.com/petems/pr-wrangler/pull/107",
			RepoNameWithOwner: "petems/pr-wrangler",
			Author:            "doc-bot[bot]",
			HeadRefName:       "docs/config-loading",
			State:             github.PRStateOpen,
			Mergeable:         string(github.MergeableMergeable),
			CreatedAt:         now.Add(-6 * time.Hour),
			UpdatedAt:         now.Add(-30 * time.Minute),
		},
		{
			Number:            108,
			Title:             "Bump dependencies and tidy go.mod",
			URL:               "https://github.com/petems/pr-wrangler/pull/108",
			RepoNameWithOwner: "petems/pr-wrangler",
			Author:            "contributor3",
			HeadRefName:       "chore/bump-deps",
			State:             github.PRStateOpen,
			Mergeable:         string(github.MergeableMergeable),
			CreatedAt:         now.Add(-36 * time.Hour),
			UpdatedAt:         now.Add(-3 * time.Hour),
			ReviewDecision:    github.ReviewDecisionReviewRequired,
			LatestCheckState:  string(github.CIConclusionSuccess),
			StatusChecks: []github.StatusCheck{
				{Name: "test", Conclusion: github.CIConclusionSuccess, Status: "COMPLETED"},
			},
			Reviews: []github.Review{
				{Author: "octocat", State: "COMMENTED"},
			},
		},
		{
			Number:            109,
			Title:             "Closed: superseded by #108",
			URL:               "https://github.com/petems/pr-wrangler/pull/109",
			RepoNameWithOwner: "petems/pr-wrangler",
			Author:            "petems",
			HeadRefName:       "chore/old-bump",
			State:             github.PRStateClosed,
			CreatedAt:         now.Add(-48 * time.Hour),
			UpdatedAt:         now.Add(-40 * time.Hour),
		},
		{
			Number:            110,
			Title:             "Initial repository scaffolding and Makefile targets",
			URL:               "https://github.com/petems/pr-wrangler/pull/110",
			RepoNameWithOwner: "petems/pr-wrangler",
			Author:            "petems",
			HeadRefName:       "feat/initial-scaffolding",
			State:             github.PRStateMerged,
			MergedAt:          &mergedAt,
			CreatedAt:         now.Add(-7 * 24 * time.Hour),
			UpdatedAt:         mergedAt,
		},
	}
}

// MockSAMLErrors returns SAML placeholder entries that exercise the
// "SAML Auth Required" rendering path. Index values are positions in the
// pre-interleave search ordering so buildRows can splice them back into
// the row list.
func MockSAMLErrors() []github.SAMLErrorEntry {
	return []github.SAMLErrorEntry{
		{
			Index:             3,
			RepoNameWithOwner: "internal-org/private-service",
			PRNumber:          42,
			Err: &github.SAMLAuthError{
				Message: "Resource protected by organization SAML",
				AuthURL: "https://github.com/enterprises/internal-org/sso?authorization_request=DEMO123",
			},
		},
		{
			Index:             7,
			RepoNameWithOwner: "internal-org/billing-platform",
			PRNumber:          17,
			Err: &github.SAMLAuthError{
				Message: "Resource protected by organization SAML",
				AuthURL: "https://github.com/enterprises/internal-org/sso?authorization_request=DEMO456",
			},
		},
	}
}

// MockPRSessions returns example active tmux sessions keyed by PR number,
// matching a subset of the PRs returned by MockPRs.
func MockPRSessions() map[int]tmux.PRSession {
	return map[int]tmux.PRSession{
		101: {
			SessionName: "pr-101-add-oauth-device-flow",
			PRNumber:    101,
			PRTitle:     "Add OAuth device flow for GitHub authentication",
			WorkDir:     "/Users/demo/projects/pr-wrangler",
			Branch:      "feat/oauth-device-flow",
			PRURL:       "https://github.com/petems/pr-wrangler/pull/101",
		},
		102: {
			SessionName: "pr-102-migrate-tui-to-charm-v2",
			PRNumber:    102,
			PRTitle:     "Migrate TUI to Charm v2 to fix OSC 8 hyperlink truncation",
			WorkDir:     "/Users/demo/projects/pr-wrangler-worktrees/pr-102",
			Branch:      "fix/charm-v2-migration",
			PRURL:       "https://github.com/petems/pr-wrangler/pull/102",
		},
		104: {
			SessionName: "pr-104-add-real-time-colour-scheme",
			PRNumber:    104,
			PRTitle:     "Add real-time colour scheme switching with theme picker overlay",
			WorkDir:     "/Users/demo/projects/pr-wrangler-worktrees/pr-104",
			Branch:      "feat/theme-picker",
			PRURL:       "https://github.com/petems/pr-wrangler/pull/104",
		},
	}
}
