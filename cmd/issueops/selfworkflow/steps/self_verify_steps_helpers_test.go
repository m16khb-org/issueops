package steps

import (
	application "issueops/internal/application/selfverify"
	contract "issueops/internal/contract/selfverify"
)

type StepResult = contract.StepResult
type SelfVerifyPlannedStep = application.SelfVerifyPlannedStep
type RiskQAEvidence = application.RiskQAEvidence
type SelfVerifyStepDeps = application.SelfVerifyStepDeps

func PlannedSelfVerifySteps(root, tempBin string, seed int64, goTestStep *StepResult, deps SelfVerifyStepDeps) []SelfVerifyPlannedStep {
	return application.PlannedSteps(root, tempBin, seed, goTestStep, deps)
}
func CachedContractGoldenStep(goTestStep StepResult, deps SelfVerifyStepDeps) StepResult {
	return application.CachedContractGoldenStep(goTestStep, deps)
}
