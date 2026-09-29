package probe

import "issueops/internal/adapter/verification/probe/nativeintegration"

type ClaudeMCPDuplicateWarning = nativeintegration.ClaudeMCPDuplicateWarning

func validateNativeIntegration(root string) StepResult {
	return nativeintegration.Validate(root)
}
