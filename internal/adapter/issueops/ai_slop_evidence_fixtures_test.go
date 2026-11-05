package issueops

import (
	reviewapp "issueops/internal/application/issueopsreview"
	"issueops/internal/contract/issueops"
	reviewport "issueops/internal/port/issueopsreview"
)

// RecordIssueOpsAISlopCleanEvidence persists which cleanup categories were
// checked/cleaned and which verifications were rerun — the source of truth
// backing the ai-slop-clean cleanup_evidence and verification_evidence artifacts.
func RecordIssueOpsAISlopCleanEvidence(stateRoot, id string, categories, verification []string) (issueops.Record, error) {
	return recordIssueOpsAISlopCleanEvidence(stateRoot, id, categories, verification, nil)
}

func recordIssueOpsAISlopCleanEvidence(stateRoot, id string, categories, verification []string, actor *issueops.Actor) (issueops.Record, error) {
	return reviewapp.RecordAISlopCleanEvidence(reviewport.AISlopCleanStore{
		ReviewMutationStore: NewReviewMutationStore(actor),
		Refresh:             testCyclePhaseService(actor).Refresh,
	}, stateRoot, id, categories, verification)
}
