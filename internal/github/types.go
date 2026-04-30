package github

import (
	"fmt"
	"strings"
	"time"
)

type PRState string

const (
	PRStateOpen   PRState = "OPEN"
	PRStateClosed PRState = "CLOSED"
	PRStateMerged PRState = "MERGED"
)

type ReviewDecision string

const (
	ReviewDecisionApproved         ReviewDecision = "APPROVED"
	ReviewDecisionChangesRequested ReviewDecision = "CHANGES_REQUESTED"
	ReviewDecisionReviewRequired   ReviewDecision = "REVIEW_REQUIRED"
)

type CIConclusion string

const (
	CIConclusionSuccess CIConclusion = "SUCCESS"
	CIConclusionFailure CIConclusion = "FAILURE"
	CIConclusionSkipped CIConclusion = "SKIPPED"
)

type StatusCheck struct {
	Name       string
	Conclusion CIConclusion
	Status     string
	DetailsURL string
}

type Review struct {
	Author string
	State  string
}

type Mergeable string

const (
	MergeableConflicting Mergeable = "CONFLICTING"
	MergeableMergeable   Mergeable = "MERGEABLE"
	MergeableUnknown     Mergeable = "UNKNOWN"
)

type PR struct {
	Number            int
	Title             string
	URL               string
	RepoNameWithOwner string
	Author            string
	HeadRefName       string
	HeadCommitOID     string
	State             PRState
	IsDraft           bool
	Mergeable         string
	Labels            []string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	MergedAt          *time.Time
	ReviewDecision    ReviewDecision
	UnresolvedThreads int
	LatestCheckState  string
	StatusChecks      []StatusCheck
	Reviews           []Review
}

// ExtractServiceNames finds service names from labels using the provided prefix.
func ExtractServiceNames(labels []string, prefix string) []string {
	var services []string
	for _, l := range labels {
		if strings.HasPrefix(l, prefix) {
			services = append(services, strings.TrimPrefix(l, prefix))
		}
	}
	return services
}

type PRStatus string

func (s PRStatus) String() string { return string(s) }

const (
	PRStatusMerged               PRStatus = "Merged"
	PRStatusDraft                PRStatus = "Draft"
	PRStatusDraftCIFailing       PRStatus = "Draft (CI failing)"
	PRStatusCIFailing            PRStatus = "CI failing"
	PRStatusChangesRequested     PRStatus = "Changes requested"
	PRStatusWaitingForReviews    PRStatus = "Waiting for reviews"
	PRStatusApproved             PRStatus = "Approved"
	PRStatusReviewedWithComments PRStatus = "Reviewed (comments)"
	PRStatusOpen                 PRStatus = "Open"
	PRStatusHasConflicts         PRStatus = "Has conflicts"
	PRStatusSAMLRequired         PRStatus = "SAML Auth Required"
)

type Action string

func (a Action) String() string { return string(a) }

const (
	ActionFixCI            Action = "Fix CI"
	ActionAddressFeedback  Action = "Address feedback"
	ActionMerge            Action = "Merge"
	ActionInvestigate      Action = "Investigate"
	ActionResolveConflicts Action = "Resolve conflicts"
	ActionReviewComments   Action = "Review comments"
	ActionAuthorizeSAML    Action = "Authorize SAML"
	ActionNone             Action = ""
)

func (a Action) IsActionable() bool {
	return a != ActionNone
}

func IsBotAuthor(author string) bool {
	author = strings.ToLower(author)
	bots := []string{"datadog-official", "chatgpt-codex-connector", "github-actions"}
	for _, bot := range bots {
		if author == bot {
			return true
		}
	}
	return strings.HasSuffix(author, "[bot]")
}

// SAMLAuthError represents a 403 error caused by SAML SSO requirement.
type SAMLAuthError struct {
	Message       string
	AuthURL       string
	OriginalError error
}

func (e *SAMLAuthError) Error() string {
	if e.AuthURL != "" {
		return fmt.Sprintf("%s (authorize at: %s)", e.Message, e.AuthURL)
	}
	return e.Message
}

func (e *SAMLAuthError) Unwrap() error {
	return e.OriginalError
}

// SAMLErrorEntry pairs a SAML auth error with its original search-result
// position so callers can interleave SAML rows back into the PR list at the
// correct sequence instead of appending them in non-deterministic order.
type SAMLErrorEntry struct {
	Index             int
	RepoNameWithOwner string
	PRNumber          int
	Err               *SAMLAuthError
}
