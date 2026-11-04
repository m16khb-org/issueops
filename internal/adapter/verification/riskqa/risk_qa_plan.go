package riskqa

import (
	riskqacontract "issueops/internal/contract/riskqa"

	"encoding/json"
	"fmt"

	riskqaapp "issueops/internal/application/riskqa"
)

func Plan(root string) riskqacontract.TierPlan {
	return PlanWithScope(root, "")
}

func PlanWithScope(root, baseRef string) riskqacontract.TierPlan {
	return (riskqaapp.Service{ScopedPaths: gitScopedPaths}).PlanWithScope(root, baseRef)
}

func PlanJSON(plan riskqacontract.TierPlan) string {
	b, err := json.Marshal(plan)
	if err != nil {
		return fmt.Sprintf(`{"tier":%q,"error":%q}`, plan.Tier, err.Error())
	}
	return string(b)
}
