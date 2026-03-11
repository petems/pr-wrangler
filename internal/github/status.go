package github

func DetermineStatus(pr PR) PRStatus {
	if pr.State == PRStateMerged {
		return PRStatusMerged
	}

	if pr.Mergeable == string(MergeableConflicting) {
		return PRStatusHasConflicts
	}

	hasCIFailure := false
	for _, check := range pr.StatusChecks {
		if check.Conclusion == CIConclusionFailure {
			hasCIFailure = true
			break
		}
	}

	// Unresolved threads or latest review being changes_requested
	hasChangesRequested := pr.UnresolvedThreads > 0
	if !hasChangesRequested {
		for _, rev := range pr.Reviews {
			if rev.State == "CHANGES_REQUESTED" {
				hasChangesRequested = true
				break
			}
		}
	}

	if hasChangesRequested {
		return PRStatusChangesRequested
	}

	if hasCIFailure {
		if pr.IsDraft {
			return PRStatusDraftCIFailing
		}
		return PRStatusCIFailing
	}

	if pr.IsDraft {
		return PRStatusDraft
	}

	if len(pr.Reviews) == 0 {
		return PRStatusWaitingForReviews
	}

	allApproved := true
	anyComment := false
	for _, rev := range pr.Reviews {
		if rev.State == "APPROVED" {
			continue
		}
		allApproved = false
		if rev.State == "COMMENTED" {
			anyComment = true
		}
	}

	if allApproved {
		return PRStatusApproved
	}

	if anyComment {
		return PRStatusReviewedWithComments
	}

	return PRStatusOpen
}
