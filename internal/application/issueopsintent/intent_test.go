package issueopsintent

import (
	"reflect"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	intentport "issueops/internal/port/issueopsintent"
)

func TestRecordIntentRejectsWeakRequestBeforeRead(t *testing.T) {
	store := intentport.Store{Read: func(_, _ string) (model.IssueOpsRecord, error) {
		t.Fatal("weak intent should fail before read")
		return model.IssueOpsRecord{}, nil
	}}
	_, err := RecordIntent(store, "state", "io-1", model.IssueOpsIntentRecordRequest{RawRequest: "raw"})
	if err == nil || !strings.Contains(err.Error(), "interpreted_intent is required") {
		t.Fatalf("err=%v", err)
	}
}

func TestRecordIntentRejectsSecretBeforeWriteAndKeepsOrder(t *testing.T) {
	var events []string
	store := intentport.Store{
		Read: func(_, _ string) (model.IssueOpsRecord, error) {
			events = append(events, "read")
			return model.IssueOpsRecord{ID: "io-1"}, nil
		},
		Now: func() string { events = append(events, "clock"); return "2026-09-28T00:00:00Z" },
		TouchWrite: func(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			events = append(events, "write")
			return record, nil
		},
	}
	record, err := RecordIntent(store, "state", "io-1", model.IssueOpsIntentRecordRequest{
		RawRequest: "fix quality gate", InterpretedIntent: "stabilize quality gate tests",
		SuccessCriteria: []string{"go test"}, IntentClass: "architecture",
	})
	if err != nil || !reflect.DeepEqual(events, []string{"read", "clock", "write"}) ||
		record.Intent == nil || record.Intent.RecordedAt != "2026-09-28T00:00:00Z" {
		t.Fatalf("record=%+v err=%v events=%v", record, err, events)
	}
	events = nil
	_, err = RecordIntent(store, "state", "io-1", model.IssueOpsIntentRecordRequest{
		RawRequest: "fix quality gate", InterpretedIntent: "stabilize quality gate tests",
		SuccessCriteria: []string{"go test"}, Constraints: []string{"claim token: abc123 must never be printed"},
	})
	if err == nil || !strings.Contains(err.Error(), "secret-like") || !reflect.DeepEqual(events, []string{"read", "clock"}) {
		t.Fatalf("err=%v events=%v", err, events)
	}
}
