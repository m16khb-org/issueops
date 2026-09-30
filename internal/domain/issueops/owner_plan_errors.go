package issueops

type OwnerPlanRequiredError struct {
	NextCommand string
}

func (e *OwnerPlanRequiredError) Error() string {
	return "Orca execution requires a staged plan artifact"
}

func (e *OwnerPlanRequiredError) IssueOpsErrorFields() map[string]any {
	fields := map[string]any{
		"code":    "orca_plan_artifact_required",
		"missing": []string{"plan"},
	}
	if e.NextCommand != "" {
		fields["next_command"] = e.NextCommand
	}
	return fields
}

// OwnerPlanReviewStaleError: the recorded verdict describes a different plan than
// the one being sealed for the owner. Caught here so the planner fixes it before
// an owner is launched into a gate it cannot fill (#319).
type OwnerPlanReviewStaleError struct {
	NextCommand string
}

func (e *OwnerPlanReviewStaleError) Error() string {
	return "devil's-advocate verdict was recorded against a different plan; re-run the review on the plan being staged"
}

func (e *OwnerPlanReviewStaleError) IssueOpsErrorFields() map[string]any {
	return map[string]any{
		"code":         "devils_advocate_review_stale",
		"missing":      []string{"devils_advocate_review_stale"},
		"next_command": e.NextCommand,
	}
}
