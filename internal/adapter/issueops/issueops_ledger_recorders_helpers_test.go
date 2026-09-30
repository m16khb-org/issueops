package issueops

import (
	reviewapp "issueops/internal/application/issueopsreview"
	"issueops/internal/contract/issueops"
)

// RecordIssueOpsDomainReview persists the grill-phase domain review
// (terminology, model fit, risks, uncertainties) — the source of truth backing
// the grill domain_review artifact.
func RecordIssueOpsDomainReview(stateRoot, id string, req issueops.IssueOpsDomainReviewRequest) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsDomainReview(stateRoot, id, req, nil)
}

func RecordIssueOpsDomainReviewWithActor(stateRoot, id string, req issueops.IssueOpsDomainReviewRequest, actor issueops.IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsDomainReview(stateRoot, id, req, &actor)
}

func recordIssueOpsDomainReview(stateRoot, id string, req issueops.IssueOpsDomainReviewRequest, actor *issueops.IssueOpsActor) (issueops.IssueOpsRecord, error) {
	store := NewReviewMutationStore(actor)
	store.ValidateMutation = func(record issueops.IssueOpsRecord) error {
		return validateWorkspacePreparationMutation(record, actor)
	}
	return reviewapp.RecordDomainReview(store, stateRoot, id, req)
}

// ResolveIssueOpsFeedback records the outcome of a feedback item by index — the
// source of truth backing the feedback feedback_resolution artifact.
func ResolveIssueOpsFeedback(stateRoot, id string, index int, resolution string) (issueops.IssueOpsRecord, error) {
	return resolveIssueOpsFeedback(stateRoot, id, index, resolution, nil)
}

func ResolveIssueOpsFeedbackWithActor(stateRoot, id string, index int, resolution string, actor issueops.IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return resolveIssueOpsFeedback(stateRoot, id, index, resolution, &actor)
}

func resolveIssueOpsFeedback(stateRoot, id string, index int, resolution string, actor *issueops.IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return reviewapp.ResolveFeedback(NewReviewMutationStore(actor), stateRoot, id, index, resolution)
}
