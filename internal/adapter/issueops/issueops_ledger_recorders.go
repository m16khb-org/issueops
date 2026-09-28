package issueops

import (
	reviewapp "issueops/internal/application/issueopsreview"
	"issueops/internal/contract/issueops"
	reviewport "issueops/internal/port/issueopsreview"
)

// RecordIssueOpsDomainReview persists the grill-phase domain review
// (terminology, model fit, risks, uncertainties) — the source of truth backing
// the grill domain_review artifact.
func RecordIssueOpsDomainReview(stateRoot, id string, req issueops.IssueOpsDomainReviewRequest) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsDomainReview(stateRoot, id, req, nil)
}

func RecordIssueOpsDomainReviewWithActor(stateRoot, id string, req issueops.IssueOpsDomainReviewRequest, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsDomainReview(stateRoot, id, req, &actor)
}

func recordIssueOpsDomainReview(stateRoot, id string, req issueops.IssueOpsDomainReviewRequest, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	store := reviewMutationStore(actor)
	store.ValidateMutation = func(record issueops.IssueOpsRecord) error {
		return validateWorkspacePreparationMutation(record, actor)
	}
	return reviewapp.RecordDomainReview(store, stateRoot, id, req)
}

// RecordIssueOpsAISlopCleanEvidence persists which cleanup categories were
// checked/cleaned and which verifications were rerun — the source of truth
// backing the ai-slop-clean cleanup_evidence and verification_evidence artifacts.
func RecordIssueOpsAISlopCleanEvidence(stateRoot, id string, categories, verification []string) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsAISlopCleanEvidence(stateRoot, id, categories, verification, nil)
}

func RecordIssueOpsAISlopCleanEvidenceWithActor(stateRoot, id string, categories, verification []string, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsAISlopCleanEvidence(stateRoot, id, categories, verification, &actor)
}

func recordIssueOpsAISlopCleanEvidence(stateRoot, id string, categories, verification []string, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return reviewapp.RecordAISlopCleanEvidence(reviewport.AISlopCleanStore{
		ReviewMutationStore: reviewMutationStore(actor),
		Refresh:             refreshIssueOpsAISlopClean,
	}, stateRoot, id, categories, verification)
}

// ResolveIssueOpsFeedback records the outcome of a feedback item by index — the
// source of truth backing the feedback feedback_resolution artifact.
func ResolveIssueOpsFeedback(stateRoot, id string, index int, resolution string) (issueops.IssueOpsRecord, error) {
	return resolveIssueOpsFeedback(stateRoot, id, index, resolution, nil)
}

func ResolveIssueOpsFeedbackWithActor(stateRoot, id string, index int, resolution string, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return resolveIssueOpsFeedback(stateRoot, id, index, resolution, &actor)
}

func resolveIssueOpsFeedback(stateRoot, id string, index int, resolution string, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return reviewapp.ResolveFeedback(reviewMutationStore(actor), stateRoot, id, index, resolution)
}
