package probe

import "issueops/internal/adapter/verification/probe/parallelisolation"

func validateParallelTempIsolation(binary, root string, seed int64) StepResult {
	return parallelisolation.Validate(binary, root, seed)
}
