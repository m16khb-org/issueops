package failurecause

import (
	failurecausecontract "issueops/internal/contract/failurecause"
	failurecausedomain "issueops/internal/domain/failurecause"
)

func Classify(failed bool, items []failurecausecontract.Evidence) failurecausecontract.Result {
	return failurecausedomain.Classify(failed, items)
}
