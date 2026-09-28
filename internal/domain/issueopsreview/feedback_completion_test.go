package issueopsreview

import (
	"reflect"
	"testing"
)

func TestFeedbackCompletionMissingPreservesGateOrder(t *testing.T) {
	items := []FeedbackSnapshot{
		{Classification: "", Resolution: ""},
		{Classification: " CONTRACT_CHANGE ", IssueUpdatedAt: " ", Resolution: "answered"},
	}
	want := []string{"feedback_classification", "contract_feedback_issue_update", "feedback_resolution"}
	if got := FeedbackCompletionMissing(items); !reflect.DeepEqual(got, want) {
		t.Fatalf("missing=%v, want %v", got, want)
	}
	if !HasUnresolvedContractFeedback(items) {
		t.Fatal("contract change without issue update must remain unresolved")
	}
	items[1].IssueUpdatedAt = "2026-09-28T00:00:00Z"
	if HasUnresolvedContractFeedback(items) {
		t.Fatal("issue update should resolve remote update gate")
	}
}
