package issueopsapp

import (
	riskqax "issueops/internal/application/riskqa"
	riskqaxx "issueops/internal/contract/riskqa"
	selfverify "issueops/internal/contract/selfverify"

	"issueops/internal/adapter/verification/riskqa"
	riskqadomain "issueops/internal/domain/riskqa"
)

func validateRiskQATier(root string) selfverify.StepResult {
	return riskqa.Validate(root)
}

func validateRiskQATierWithDeps(root string, deps riskQATierDeps) selfverify.StepResult {
	return riskqa.ValidateWithDeps(root, riskqax.ExecuteDeps{Plan: deps.plan, Run: deps.run})
}

func planRiskQATier(root string) RiskQATierPlan {
	return riskqa.Plan(root)
}

func planRiskQATierFromPaths(paths []string) RiskQATierPlan {
	return riskqadomain.PlanFromPaths(paths)
}

func parseGitStatusPath(line string) string {
	return riskqa.ParseGitStatusPath(line)
}

func riskQATierPlanJSON(plan RiskQATierPlan) string {
	return riskqa.PlanJSON(plan)
}

type riskQATierDeps struct {
	plan func(string) RiskQATierPlan
	run  func(root string, command string) selfverify.StepResult
}

type RiskQATierPlan = riskqaxx.RiskQATierPlan
