package issueops

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestIntentMissingPreservesOrderedEvidenceKeys(t *testing.T) {
	if got := IntentMissing(model.IssueOpsRecord{}); !reflect.DeepEqual(got, []string{"intent_contract"}) {
		t.Fatalf("missing intent=%v", got)
	}
	record := model.IssueOpsRecord{Intent: &model.IssueOpsIntentContract{RawRequest: " ", InterpretedIntent: "", SuccessCriteria: []string{"\x00"}}}
	if got := IntentMissing(record); !reflect.DeepEqual(got, []string{"raw_request", "interpreted_intent", "success_criteria"}) {
		t.Fatalf("incomplete intent=%v", got)
	}
	record.Intent.RawRequest = "request"
	record.Intent.InterpretedIntent = "outcome"
	record.Intent.SuccessCriteria = []string{"go test"}
	if got := IntentMissing(record); len(got) != 0 {
		t.Fatalf("complete intent missing=%v", got)
	}
}
