package riskqa

import (
	"sort"

	riskqacontract "issueops/internal/contract/riskqa"
	riskqadomain "issueops/internal/domain/riskqa"
)

type Service struct {
	ChangedPaths func(root string) ([]string, []string)
	ScopedPaths  func(root, baseRef string) ([]string, *riskqacontract.Scope)
}

func (service Service) Plan(root string) riskqacontract.RiskQATierPlan {
	paths, warnings := service.ChangedPaths(root)
	plan := riskqadomain.PlanFromPaths(paths)
	plan.Reasons = append(plan.Reasons, warnings...)
	sort.Strings(plan.Reasons)
	return plan
}

func (service Service) PlanWithScope(root, baseRef string) riskqacontract.RiskQATierPlan {
	paths, scope := service.ScopedPaths(root, baseRef)
	plan := riskqadomain.PlanFromPaths(paths)
	plan.Scope = scope
	return plan
}
