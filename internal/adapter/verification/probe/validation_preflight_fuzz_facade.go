package probe

import "issueops/internal/adapter/verification/probe/preflightfuzz"

func validatePreflightFuzz(binary, root string, seed int64) StepResult {
	return preflightfuzz.Validate(binary, root, seed)
}
