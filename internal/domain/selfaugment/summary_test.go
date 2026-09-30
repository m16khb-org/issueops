package selfaugment

import "testing"

func TestEmptySummaryDoesNotGainPerfectScore(t *testing.T) {
	summary := SummarizeSteps(nil, 95)
	FinalizeSummary(&summary, false)
	if summary.TotalRuns != 0 || summary.MinimumGoalScore != 0 || summary.TerminationEligible {
		t.Fatalf("empty summary gained success: %+v", summary)
	}
}
