package issueops

import (
	"context"
	reviewapp "issueops/internal/application/issueopsreview"
	"issueops/internal/contract/issueops"
)

// RecordIssueOpsDomainReview persists the grill-phase domain review
// (terminology, model fit, risks, uncertainties) — the source of truth backing
// the grill domain_review artifact.
func RecordIssueOpsDomainReview(stateRoot, id string, req issueops.DomainReviewRequest) (issueops.Record, error) {
	return recordIssueOpsDomainReview(stateRoot, id, req, nil)
}

func recordIssueOpsDomainReview(stateRoot, id string, req issueops.DomainReviewRequest, actor *issueops.Actor) (issueops.Record, error) {
	store := NewReviewMutationStore(actor)
	store.ValidateMutation = func(record issueops.Record) error {
		return validateExecutionMutation(context.Background(), record, actor, liveTestVerifier())
	}
	return reviewapp.RecordDomainReview(store, stateRoot, id, req)
}

// ResolveIssueOpsFeedback records the outcome of a feedback item by index — the
// source of truth backing the feedback feedback_resolution artifact.
func ResolveIssueOpsFeedback(stateRoot, id string, index int, resolution string) (issueops.Record, error) {
	return resolveIssueOpsFeedback(stateRoot, id, index, resolution, nil)
}

func resolveIssueOpsFeedback(stateRoot, id string, index int, resolution string, actor *issueops.Actor) (issueops.Record, error) {
	return reviewapp.ResolveFeedback(NewReviewMutationStore(actor), stateRoot, id, index, resolution)
}
