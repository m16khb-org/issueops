package riskqa

import (
	riskqacontract "issueops/internal/contract/riskqa"

	"encoding/json"
	"fmt"

	riskqaapp "issueops/internal/application/riskqa"
)

// Plan returns the risk QA tier plan for root without a base ref scope.
func Plan(root string) riskqacontract.TierPlan {
	return planWithScope(root, "")
}

func planWithScope(root, baseRef string) riskqacontract.TierPlan {
	return (riskqaapp.Service{ScopedPaths: gitScopedPaths}).PlanWithScope(root, baseRef)
}

// PlanJSON renders plan as compact JSON. On a marshal failure it returns a JSON
// object carrying the tier and the error message.
func PlanJSON(plan riskqacontract.TierPlan) string {
	b, err := json.Marshal(plan)
	if err != nil {
		return fmt.Sprintf(`{"tier":%q,"error":%q}`, plan.Tier, err.Error())
	}
	return string(b)
}
