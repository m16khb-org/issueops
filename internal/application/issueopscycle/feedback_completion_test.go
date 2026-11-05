package issueopscycle

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestFeedbackCompletionMissingMapsPersistedFeedback(t *testing.T) {
	record := model.Record{Feedback: []model.FeedbackItem{
		{Classification: "contract_change", Resolution: "fixed"},
		{Classification: "question", Resolution: ""},
	}}
	want := []string{"contract_feedback_issue_update", "feedback_resolution"}
	if got := feedbackCompletionMissing(record); !reflect.DeepEqual(got, want) {
		t.Fatalf("missing=%v, want %v", got, want)
	}
	if !hasUnresolvedContractFeedback(record) {
		t.Fatal("contract change should require issue update")
	}
}
