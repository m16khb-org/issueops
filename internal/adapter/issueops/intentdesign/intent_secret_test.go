package intentdesign

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestRecordIntentRejectsColonFormSecretBeforeWriting(t *testing.T) {
	store, rec := planPrepMemStore()
	_, err := RecordIntent(store, "state", "io-1", model.IssueOpsIntentRecordRequest{
		RawRequest:        "raw ask",
		InterpretedIntent: "reframed agent interpretation differs",
		SuccessCriteria:   []string{"works"},
		Constraints:       []string{"claim token: abc123 must never be printed"},
	})
	if err == nil || !strings.Contains(err.Error(), "secret-like") {
		t.Fatalf("colon-form secret must be rejected at record time: %v", err)
	}
	if rec.Intent != nil {
		t.Fatalf("rejected intent must not be written: %#v", rec.Intent)
	}
}

func TestRecordIntentKeepsRedactingEqualsFormSecret(t *testing.T) {
	store, _ := planPrepMemStore()
	rec, err := RecordIntent(store, "state", "io-1", model.IssueOpsIntentRecordRequest{
		RawRequest:        "remove password=hunter2 from the logs",
		InterpretedIntent: "reframed agent interpretation differs",
		SuccessCriteria:   []string{"works"},
	})
	if err != nil {
		t.Fatalf("equals-form secret must keep the redaction path: %v", err)
	}
	if rec.Intent == nil || rec.Intent.RawRequest != "<redacted>" {
		t.Fatalf("equals-form secret must be stored as <redacted>: %#v", rec.Intent)
	}
}
