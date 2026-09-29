package probe

import "issueops/internal/adapter/verification/probe/smoke"

func validateInspect(binary, root string) StepResult {
	return smoke.ValidateInspect(binary, root)
}

func validateDocsIndex(binary, root string) StepResult {
	return smoke.ValidateDocsIndex(binary, root)
}
