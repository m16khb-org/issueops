package probe

import (
	selfverify "issueops/internal/contract/selfverify"
)

import "issueops/internal/adapter/verification/probe/smoke"

func validateInspect(binary, root string) selfverify.StepResult {
	return smoke.ValidateInspect(binary, root)
}

func validateDocsIndex(binary, root string) selfverify.StepResult {
	return smoke.ValidateDocsIndex(binary, root)
}
