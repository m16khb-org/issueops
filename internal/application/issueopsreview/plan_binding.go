package issueopsreview

import (
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueopsreview"
)

type planDigestSource interface {
	LinkedDigest(model.Record) (string, error)
	StagedPlans(string, string) (map[string]string, error)
}

// PlanDigestResolver resolves the digest of the plan a review is bound to:
// the linked plan file when the record has one, otherwise the staged plan.
type PlanDigestResolver struct{ source planDigestSource }

// NewPlanDigestResolver returns a PlanDigestResolver that reads plans from source.
func NewPlanDigestResolver(source planDigestSource) PlanDigestResolver {
	return PlanDigestResolver{source: source}
}

// Digest returns the digest of the plan bound to record: its linked plan when
// PlanPath is set, otherwise the plan staged under stateRoot for the record id.
func (r PlanDigestResolver) Digest(stateRoot string, record model.Record) (string, error) {
	if domain.HasLinkedReviewPlan(record.PlanPath) {
		return r.source.LinkedDigest(record)
	}
	staged, err := r.source.StagedPlans(stateRoot, record.ID)
	if err != nil {
		return domain.StagedReviewPlanDigest("")
	}
	return domain.StagedReviewPlanDigest(staged["plan"])
}
