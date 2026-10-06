package riskqa

import (
	riskqacontract "issueops/internal/contract/riskqa"
	riskqadomain "issueops/internal/domain/riskqa"
)

type Service struct {
	ScopedPaths func(root, baseRef string) ([]string, *riskqacontract.Scope)
}

func (service Service) PlanWithScope(root, baseRef string) riskqacontract.RiskQATierPlan {
	paths, scope := service.ScopedPaths(root, baseRef)
	plan := riskqadomain.PlanFromPaths(paths)
	plan.Scope = scope
	return plan
}
