package probe

import (
	selfverify "issueops/internal/contract/selfverify"
)

import "issueops/internal/adapter/verification/probe/daemonresilience"

func validateDaemonRestartResilience(binary, root string, seed int64) selfverify.StepResult {
	return daemonresilience.Validate(binary, root, seed)
}
