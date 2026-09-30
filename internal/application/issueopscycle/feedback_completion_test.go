package issueopscycle

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestFeedbackCompletionMissingMapsPersistedFeedback(t *testing.T) {
	record := model.IssueOpsRecord{Feedback: []model.IssueOpsFeedbackItem{
		{Classification: "contract_change", Resolution: "fixed"},
		{Classification: "question", Resolution: ""},
	}}
	want := []string{"contract_feedback_issue_update", "feedback_resolution"}
	if got := FeedbackCompletionMissing(record); !reflect.DeepEqual(got, want) {
		t.Fatalf("missing=%v, want %v", got, want)
	}
	if !HasUnresolvedContractFeedback(record) {
		t.Fatal("contract change should require issue update")
	}
}
