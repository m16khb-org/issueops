package issueopsapp

import (
	riskqax "issueops/internal/application/riskqa"
	riskqaxx "issueops/internal/contract/riskqa"
	selfverify "issueops/internal/contract/selfverify"

	"issueops/internal/adapter/verification/riskqa"
)

func validateRiskQATierWithDeps(root string, deps riskQATierDeps) selfverify.StepResult {
	return riskqa.ValidateWithDeps(root, riskqax.ExecuteDeps{Plan: deps.plan, Run: deps.run})
}

type riskQATierDeps struct {
	plan func(string) riskqaxx.RiskQATierPlan
	run  func(root string, command string) selfverify.StepResult
}
