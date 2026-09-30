package probe

import (
	selfverify "issueops/internal/contract/selfverify"
)

import "issueops/internal/adapter/verification/probe/parallelisolation"

func validateParallelTempIsolation(binary, root string, seed int64) selfverify.StepResult {
	return parallelisolation.Validate(binary, root, seed)
}
