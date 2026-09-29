package issueopsapp

import (
	"issueops/internal/adapter/verification/probe/stepbudget"
)

func validateStateRoundtrip(binary, root string, seed int64) StepResult {
	return newStateRoundtripProbe().Validate(binary, root, seed)
}
func validateStepBudgetBaseline(binary, root string, seed int64) StepResult {
	return stepbudget.ValidateStepBudgetBaselineWithDeps(binary, root, seed, newStepBudgetProbe())
}
func validateQAGate(root string) StepResult { return newDocsQAProbe().Validate(root) }

func validateRedactionAudit(root string) StepResult { return newDocsQAProbe().RedactionAudit(root) }
