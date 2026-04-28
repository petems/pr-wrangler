package github

func DetermineAction(status PRStatus) Action {
	switch status {
	case PRStatusCIFailing, PRStatusDraftCIFailing:
		return ActionFixCI
	case PRStatusChangesRequested:
		return ActionAddressFeedback
	case PRStatusApproved:
		return ActionMerge
	case PRStatusHasConflicts:
		return ActionResolveConflicts
	case PRStatusReviewedWithComments:
		return ActionReviewComments
	default:
		return ActionNone
	}
}
