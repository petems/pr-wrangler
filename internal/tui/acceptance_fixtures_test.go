//go:build acceptance

package tui

import (
	"strconv"
	"time"

	"github.com/petems/pr-wrangler/internal/github"
)

func acceptancePROpenCIFailing() github.PR {
	return acceptancePR(101, "Fix payment retry", github.PRStateOpen, func(pr *github.PR) {
		pr.StatusChecks = []github.StatusCheck{{Name: "test", Conclusion: github.CIConclusionFailure}}
	})
}

func acceptancePRApprovedMergeable() github.PR {
	return acceptancePR(102, "Add billing export", github.PRStateOpen, func(pr *github.PR) {
		pr.Mergeable = string(github.MergeableMergeable)
		pr.Reviews = []github.Review{{Author: "reviewer", State: "APPROVED"}}
		pr.StatusChecks = []github.StatusCheck{{Name: "test", Conclusion: github.CIConclusionSuccess}}
	})
}

func acceptancePRChangesRequested() github.PR {
	return acceptancePR(103, "Tighten auth checks", github.PRStateOpen, func(pr *github.PR) {
		pr.Reviews = []github.Review{{Author: "reviewer", State: "CHANGES_REQUESTED"}}
	})
}

func acceptancePRDraft() github.PR {
	return acceptancePR(104, "Draft dashboard refresh", github.PRStateOpen, func(pr *github.PR) {
		pr.IsDraft = true
	})
}

func acceptancePRMerged() github.PR {
	mergedAt := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	return acceptancePR(105, "Merged cleanup", github.PRStateMerged, func(pr *github.PR) {
		pr.MergedAt = &mergedAt
	})
}

func acceptancePRClosed() github.PR {
	return acceptancePR(106, "Closed experiment", github.PRStateClosed, nil)
}

func acceptancePRConflicting() github.PR {
	return acceptancePR(107, "Resolve branch drift", github.PRStateOpen, func(pr *github.PR) {
		pr.Mergeable = string(github.MergeableConflicting)
	})
}

func acceptanceSAMLError(index int, repo string, number int) github.SAMLErrorEntry {
	return github.SAMLErrorEntry{
		Index:             index,
		RepoNameWithOwner: repo,
		PRNumber:          number,
		Err: &github.SAMLAuthError{
			Message: "Resource protected by organization SAML",
			AuthURL: "https://github.com/orgs/example/sso?authorization_request=abc123",
		},
	}
}

func acceptanceSAMLErrorWithoutURL(index int, repo string, number int) github.SAMLErrorEntry {
	entry := acceptanceSAMLError(index, repo, number)
	entry.Err.AuthURL = ""
	return entry
}

func acceptancePR(number int, title string, state github.PRState, mutate func(*github.PR)) github.PR {
	pr := github.PR{
		Number:            number,
		Title:             title,
		URL:               "https://github.com/example/repo/pull/" + strconv.Itoa(number),
		RepoNameWithOwner: "example/repo",
		Author:            "author",
		HeadRefName:       "feature/pr-" + strconv.Itoa(number),
		HeadCommitOID:     "abc123",
		State:             state,
		Mergeable:         string(github.MergeableMergeable),
		CreatedAt:         time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC),
		UpdatedAt:         time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC),
	}
	if mutate != nil {
		mutate(&pr)
	}
	return pr
}
