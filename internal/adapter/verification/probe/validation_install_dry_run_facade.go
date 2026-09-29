package probe

import "issueops/internal/adapter/verification/probe/installdryrun"

func validateInstallDryRunSmoke(binary, root string, seed int64) StepResult {
	return installdryrun.Validate(binary, root, seed)
}
