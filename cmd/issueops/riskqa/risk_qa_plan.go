package riskqa

import (
	"encoding/json"
	"fmt"
	"sort"

	riskqadomain "issueops/internal/domain/riskqa"
)

func Plan(root string) RiskQATierPlan {
	paths, warnings := gitChangedPaths(root)
	plan := riskqadomain.PlanFromPaths(paths)
	plan.Reasons = append(plan.Reasons, warnings...)
	sort.Strings(plan.Reasons)
	return plan
}

func PlanJSON(plan RiskQATierPlan) string {
	b, err := json.Marshal(plan)
	if err != nil {
		return fmt.Sprintf(`{"tier":%q,"error":%q}`, plan.Tier, err.Error())
	}
	return string(b)
}
