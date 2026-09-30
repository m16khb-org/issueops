package issueops

import (
	model "issueops/internal/contract/issueops"
	reviewport "issueops/internal/port/issueopsreview"
)

// NewEvidenceReviewStore supplies actor-fenced persistence and the caller's change observation.
func NewEvidenceReviewStore(actor *model.IssueOpsActor, fingerprint func(model.IssueOpsRecord) string) reviewport.EvidenceReviewStore {
	store := NewReviewMutationStore(actor)
	return reviewport.EvidenceReviewStore{
		Read: store.Read, Fingerprint: fingerprint, WithLock: store.WithLock,
		ValidateMutation: store.ValidateMutation, Write: store.Write, Now: store.Now,
	}
}
