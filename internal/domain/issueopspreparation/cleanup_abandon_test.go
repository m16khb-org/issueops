package issueopspreparation

import (
	"reflect"
	"testing"

	preparation "issueops/internal/contract/issueopspreparation"
)

func TestAbandonIntentInspectsEveryResourcePrefix(t *testing.T) {
	for _, tc := range []struct {
		stage preparation.IntentStage
		want  []preparation.IntentStage
	}{
		{preparation.IntentStageWorktree, []preparation.IntentStage{preparation.IntentStageWorktree}},
		{preparation.IntentStageRunBind, []preparation.IntentStage{preparation.IntentStageWorktree, preparation.IntentStageTerminal}},
		{preparation.IntentStageDispatch, []preparation.IntentStage{preparation.IntentStageWorktree, preparation.IntentStageTerminal, preparation.IntentStageTask, preparation.IntentStageDispatch}},
	} {
		got, err := CleanupAbandonIntentStages(tc.stage)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("stage %s: %v %v, want %v", tc.stage, got, err, tc.want)
		}
	}
	if _, err := CleanupAbandonIntentStages("unknown"); err == nil {
		t.Fatal("unknown stage authorized")
	}
}
