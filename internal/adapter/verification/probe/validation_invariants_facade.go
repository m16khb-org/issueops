package probe

import (
	"issueops/internal/adapter/verification/probe/invariants"
	selfverify "issueops/internal/contract/selfverify"
)

func validateHarnessInvariants(root string) selfverify.StepResult {
	return invariants.ValidateHarnessInvariants(root)
}
