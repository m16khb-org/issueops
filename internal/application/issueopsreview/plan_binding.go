package issueopsreview

import (
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueopsreview"
)

type PlanDigestSource interface {
	LinkedDigest(model.Record) (string, error)
	StagedPlans(string, string) (map[string]string, error)
}

type PlanDigestResolver struct{ source PlanDigestSource }

func NewPlanDigestResolver(source PlanDigestSource) PlanDigestResolver {
	return PlanDigestResolver{source: source}
}

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
