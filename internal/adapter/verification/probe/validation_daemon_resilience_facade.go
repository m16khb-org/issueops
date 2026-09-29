package probe

import "issueops/internal/adapter/verification/probe/daemonresilience"

func validateDaemonRestartResilience(binary, root string, seed int64) StepResult {
	return daemonresilience.Validate(binary, root, seed)
}
