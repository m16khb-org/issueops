package classification

import "testing"

func TestDeduplicateFindingsKeepsDistinctClasses(t *testing.T) {
	findings := []FindingKey{
		{Index: 0, FailureClass: "x", RecurringPattern: "same"},
		{Index: 1, FailureClass: "x", RecurringPattern: "same"},
		{Index: 2, FailureClass: "y", RecurringPattern: "same"},
	}
	got := DeduplicateFindings(findings)
	if len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Fatalf("unexpected indices: %v", got)
	}
}
