package issueops

import (
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestShouldRefreshAISlopCleanOnlyForLaterPhaseWithStamp(t *testing.T) {
	for _, tc := range []struct {
		name  string
		phase model.Phase
		stamp string
		to    model.Phase
		want  bool
	}{
		{name: "later phase", phase: model.PhaseFeedback, stamp: "2026-09-28T00:00:00Z", to: model.PhaseAISlopClean, want: true},
		{name: "missing stamp", phase: model.PhaseFeedback, to: model.PhaseAISlopClean},
		{name: "same phase", phase: model.PhaseAISlopClean, stamp: "2026-09-28T00:00:00Z", to: model.PhaseAISlopClean},
		{name: "other destination", phase: model.PhaseFeedback, stamp: "2026-09-28T00:00:00Z", to: model.PhasePR},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ShouldRefreshAISlopClean(model.Record{Phase: tc.phase, AISlopCleanAt: tc.stamp}, tc.to)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
