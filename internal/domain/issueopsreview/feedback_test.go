package issueopsreview

import "testing"

func TestFeedbackRequiresIssueUpdate(t *testing.T) {
	if !FeedbackRequiresIssueUpdate(" Contract_Change ", "") {
		t.Fatal("unresolved contract change must require an issue update")
	}
	if FeedbackRequiresIssueUpdate("contract_change", "2026-09-24T00:00:00Z") || FeedbackRequiresIssueUpdate("defect", "") {
		t.Fatal("resolved or non-contract feedback must not require an issue update")
	}
}
