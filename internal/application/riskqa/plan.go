package riskqa

import (
	"sort"

	riskqacontract "issueops/internal/contract/riskqa"
	riskqadomain "issueops/internal/domain/riskqa"
)

type Service struct {
	ChangedPaths func(root string) ([]string, []string)
}

func (service Service) Plan(root string) riskqacontract.RiskQATierPlan {
	paths, warnings := service.ChangedPaths(root)
	plan := riskqadomain.PlanFromPaths(paths)
	plan.Reasons = append(plan.Reasons, warnings...)
	sort.Strings(plan.Reasons)
	return plan
}
