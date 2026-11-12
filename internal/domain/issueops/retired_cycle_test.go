package issueops

import (
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestRetiredCycleNoteNamesTheRemovingCommand(t *testing.T) {
	const at = "2026-11-12T00:00:00Z"
	cases := []struct {
		name    string
		outcome string
		want    string
	}{
		{"finished", model.CycleOutcomeFinished, "cleanup removed it (outcome finished at " + at + ")"},
		{"abandoned", model.CycleOutcomeAbandoned, "cleanup removed it (outcome abandoned at " + at + ")"},
		{"pruned", model.CycleOutcomePruned, "prune removed it (outcome pruned at " + at + ")"},
		{"empty outcome", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RetiredCycleNote(model.CycleOutcome{Outcome: tc.outcome, RetainedAt: at})
			if got != tc.want {
				t.Fatalf("RetiredCycleNote(%q) = %q, want %q", tc.outcome, got, tc.want)
			}
		})
	}
}
