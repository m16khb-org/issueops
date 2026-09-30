package probe

import (
	selfverify "issueops/internal/contract/selfverify"
)

import "issueops/internal/adapter/verification/probe/installdryrun"

func validateInstallDryRunSmoke(binary, root string, seed int64) selfverify.StepResult {
	return installdryrun.Validate(binary, root, seed)
}
