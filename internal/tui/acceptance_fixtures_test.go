package tui

import (
	"fmt"
	"time"

	"github.com/petems/pr-wrangler/internal/github"
)

func acceptanceOpenCIFailingPR() github.PR {
	pr := acceptanceBasePR(101, "Fix failing payment specs")
	pr.StatusChecks = []github.StatusCheck{{Name: "ci", Status: "completed", Conclusion: github.CIConclusionFailure}}
	return pr
}

func acceptanceApprovedMergeablePR() github.PR {
	pr := acceptanceBasePR(102, "Add invoice export")
	pr.Mergeable = string(github.MergeableMergeable)
	pr.Reviews = []github.Review{{Author: "reviewer", State: "APPROVED"}}
	pr.StatusChecks = []github.StatusCheck{{Name: "ci", Status: "completed", Conclusion: github.CIConclusionSuccess}}
	return pr
}

func acceptanceChangesRequestedPR() github.PR {
	pr := acceptanceBasePR(103, "Refactor webhook retries")
	pr.Reviews = []github.Review{{Author: "reviewer", State: "CHANGES_REQUESTED"}}
	return pr
}

func acceptanceDraftPR() github.PR {
	pr := acceptanceBasePR(104, "Draft settlement importer")
	pr.IsDraft = true
	return pr
}

func acceptanceMergedPR() github.PR {
	pr := acceptanceBasePR(105, "Merged cleanup")
	pr.State = github.PRStateMerged
	mergedAt := acceptanceTime().Add(2 * time.Hour)
	pr.MergedAt = &mergedAt
	return pr
}

func acceptanceClosedPR() github.PR {
	pr := acceptanceBasePR(106, "Closed experiment")
	pr.State = github.PRStateClosed
	return pr
}

func acceptanceConflictingPR() github.PR {
	pr := acceptanceBasePR(107, "Resolve checkout conflicts")
	pr.Mergeable = string(github.MergeableConflicting)
	return pr
}

func acceptanceSAMLAuthError() *github.SAMLAuthError {
	return &github.SAMLAuthError{
		Message: "Resource protected by organization SAML",
		AuthURL: "https://github.com/orgs/acme/sso?authorization_request=ABC123",
	}
}

func acceptanceSAMLErrorEntry(index, number int) github.SAMLErrorEntry {
	return github.SAMLErrorEntry{
		Index:             index,
		RepoNameWithOwner: "acme/private-api",
		PRNumber:          number,
		Err:               acceptanceSAMLAuthError(),
	}
}

func acceptanceBasePR(number int, title string) github.PR {
	return github.PR{
		Number:            number,
		Title:             title,
		URL:               fmt.Sprintf("https://github.com/acme/widgets/pull/%d", number),
		RepoNameWithOwner: "acme/widgets",
		Author:            "alice",
		HeadRefName:       fmt.Sprintf("feature-%d", number),
		HeadCommitOID:     fmt.Sprintf("deadbeef%d", number),
		State:             github.PRStateOpen,
		Mergeable:         string(github.MergeableUnknown),
		CreatedAt:         acceptanceTime(),
		UpdatedAt:         acceptanceTime().Add(time.Duration(number) * time.Minute),
	}
}

func acceptanceTime() time.Time {
	return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
}
