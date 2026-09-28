package issueopspreparation

import (
	"fmt"

	preparationcontract "issueops/internal/contract/issueopspreparation"
)

// NextOrcaReceiptStage defines the durable stage progression after a receipt
// has passed its stage-specific validation.
func NextOrcaReceiptStage(stage preparationcontract.IntentStage) (preparationcontract.IntentStage, bool, error) {
	switch stage {
	case preparationcontract.IntentStageWorktree:
		return preparationcontract.IntentStageTerminal, false, nil
	case preparationcontract.IntentStageTerminal:
		return preparationcontract.IntentStageRun, false, nil
	case preparationcontract.IntentStageRun:
		return preparationcontract.IntentStageRunBind, false, nil
	case preparationcontract.IntentStageRunBind:
		return preparationcontract.IntentStageTask, false, nil
	case preparationcontract.IntentStageTask:
		return preparationcontract.IntentStageDispatch, false, nil
	case preparationcontract.IntentStageDispatch:
		return stage, true, nil
	default:
		return "", false, fmt.Errorf("unsupported Orca intent stage %q", stage)
	}
}
