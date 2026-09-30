package issueops

import (
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestShouldRefreshAISlopCleanOnlyForLaterPhaseWithStamp(t *testing.T) {
	for _, tc := range []struct {
		name  string
		phase model.IssueOpsPhase
		stamp string
		to    model.IssueOpsPhase
		want  bool
	}{
		{name: "later phase", phase: model.IssueOpsPhaseFeedback, stamp: "2026-09-28T00:00:00Z", to: model.IssueOpsPhaseAISlopClean, want: true},
		{name: "missing stamp", phase: model.IssueOpsPhaseFeedback, to: model.IssueOpsPhaseAISlopClean},
		{name: "same phase", phase: model.IssueOpsPhaseAISlopClean, stamp: "2026-09-28T00:00:00Z", to: model.IssueOpsPhaseAISlopClean},
		{name: "other destination", phase: model.IssueOpsPhaseFeedback, stamp: "2026-09-28T00:00:00Z", to: model.IssueOpsPhasePR},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ShouldRefreshAISlopClean(model.IssueOpsRecord{Phase: tc.phase, AISlopCleanAt: tc.stamp}, tc.to)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
