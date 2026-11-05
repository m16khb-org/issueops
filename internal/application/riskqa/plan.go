package riskqa

import (
	riskqacontract "issueops/internal/contract/riskqa"
	riskqadomain "issueops/internal/domain/riskqa"
)

// Service plans the QA tier from the paths a change touches. ScopedPaths
// returns those paths for a root and base ref, plus the scope it resolved.
type Service struct {
	ScopedPaths func(root, baseRef string) ([]string, *riskqacontract.Scope)
}

// PlanWithScope returns the QA tier plan for the paths changed in root against
// baseRef and attaches the resolved scope to the plan.
func (service Service) PlanWithScope(root, baseRef string) riskqacontract.TierPlan {
	paths, scope := service.ScopedPaths(root, baseRef)
	plan := riskqadomain.PlanFromPaths(paths)
	plan.Scope = scope
	return plan
}
