package trace

import "testing"

func TestDedupeTraceFindingsKeepsDistinctFailureCauses(t *testing.T) {
	findings := dedupeTraceFindings([]Finding{
		{
			FailureClass:     "shared_failure",
			FailureCause:     "transport",
			RecurringPattern: "same pattern",
			ProposedKnob:     "same knob",
		},
		{
			FailureClass:     "shared_failure",
			FailureCause:     "model",
			RecurringPattern: "same pattern",
			ProposedKnob:     "same knob",
		},
	})
	if len(findings) != 2 {
		t.Fatalf("distinct failure causes were deduped: %+v", findings)
	}
	if findings[0].FailureCause != "model" || findings[1].FailureCause != "transport" {
		t.Fatalf("findings were not deterministically sorted by failure cause: %+v", findings)
	}
	for _, finding := range findings {
		if finding.FailureCauseEvidence == nil {
			t.Fatalf("failure cause evidence must serialize as an array: %+v", finding)
		}
	}
}
