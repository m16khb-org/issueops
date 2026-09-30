package probe

import (
	selfverify "issueops/internal/contract/selfverify"
)

import "issueops/internal/adapter/verification/probe/commandpolicy"

func validateCommandPolicy(binary, root string) selfverify.StepResult {
	return commandpolicy.Validate(binary, root)
}
