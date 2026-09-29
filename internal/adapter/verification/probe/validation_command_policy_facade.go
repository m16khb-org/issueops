package probe

import "issueops/internal/adapter/verification/probe/commandpolicy"

func validateCommandPolicy(binary, root string) StepResult {
	return commandpolicy.Validate(binary, root)
}
