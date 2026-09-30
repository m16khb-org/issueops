package issueopspreparation

import (
	"fmt"

	preparation "issueops/internal/contract/issueopspreparation"
)

// Run creation/binding is a namespace, not an executable resource. All actual
// resources up to the sealed stage must be observed, including earlier stages.
func CleanupAbandonIntentStages(stage preparation.IntentStage) ([]preparation.IntentStage, error) {
	stages := []preparation.IntentStage{preparation.IntentStageWorktree}
	if stage == preparation.IntentStageWorktree {
		return stages, nil
	}
	stages = append(stages, preparation.IntentStageTerminal)
	switch stage {
	case preparation.IntentStageTerminal, preparation.IntentStageRun, preparation.IntentStageRunBind:
		return stages, nil
	case preparation.IntentStageTask:
		return append(stages, preparation.IntentStageTask), nil
	case preparation.IntentStageDispatch:
		return append(stages, preparation.IntentStageTask, preparation.IntentStageDispatch), nil
	default:
		return nil, fmt.Errorf("unsupported Orca cleanup intent stage %q", stage)
	}
}
