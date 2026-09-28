package issueopspreparation

import (
	"testing"

	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func TestNextOrcaReceiptStage(t *testing.T) {
	for _, tt := range []struct {
		current preparationcontract.IntentStage
		next    preparationcontract.IntentStage
		done    bool
	}{
		{preparationcontract.IntentStageWorktree, preparationcontract.IntentStageTerminal, false},
		{preparationcontract.IntentStageTerminal, preparationcontract.IntentStageRun, false},
		{preparationcontract.IntentStageRun, preparationcontract.IntentStageRunBind, false},
		{preparationcontract.IntentStageRunBind, preparationcontract.IntentStageTask, false},
		{preparationcontract.IntentStageTask, preparationcontract.IntentStageDispatch, false},
		{preparationcontract.IntentStageDispatch, preparationcontract.IntentStageDispatch, true},
	} {
		t.Run(string(tt.current), func(t *testing.T) {
			next, done, err := NextOrcaReceiptStage(tt.current)
			if err != nil || next != tt.next || done != tt.done {
				t.Fatalf("next=%q done=%v err=%v, want next=%q done=%v", next, done, err, tt.next, tt.done)
			}
		})
	}
	if _, _, err := NextOrcaReceiptStage("future"); err == nil {
		t.Fatal("unsupported stage was accepted")
	}
}
