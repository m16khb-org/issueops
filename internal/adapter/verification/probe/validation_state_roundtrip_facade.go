package probe

import "issueops/internal/adapter/verification/probe/stateroundtrip"

func validateStateRoundtrip(binary, root string, seed int64) StepResult {
	return stateroundtrip.Validate(binary, root, seed)
}
