package riskqa

import (
	"encoding/json"
	"fmt"

	riskqaapp "issueops/internal/application/riskqa"
)

func Plan(root string) RiskQATierPlan {
	return (riskqaapp.Service{ChangedPaths: gitChangedPaths}).Plan(root)
}

func PlanJSON(plan RiskQATierPlan) string {
	b, err := json.Marshal(plan)
	if err != nil {
		return fmt.Sprintf(`{"tier":%q,"error":%q}`, plan.Tier, err.Error())
	}
	return string(b)
}
