package issueopsapp

import (
	selfverify "issueops/internal/contract/selfverify"

	"issueops/internal/adapter/verification/probe/stepbudget"
)

func validateStateRoundtrip(binary, root string, seed int64) selfverify.StepResult {
	return newStateRoundtripProbe().Validate(binary, root, seed)
}
func validateStepBudgetBaseline(binary, root string, seed int64) selfverify.StepResult {
	return stepbudget.ValidateStepBudgetBaselineWithDeps(binary, root, seed, newStepBudgetProbe())
}
func validateQAGate(root string) selfverify.StepResult { return newDocsQAProbe().Validate(root) }

func validateRedactionAudit(root string) selfverify.StepResult {
	return newDocsQAProbe().RedactionAudit(root)
}
