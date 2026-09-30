package issueopsrecord

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCleanupOperationCodecBindsSupportedOperations(t *testing.T) {
	for _, operation := range []string{"finish", "remote-branch", "abandon"} {
		t.Run(operation, func(t *testing.T) {
			raw := []byte(fmt.Sprintf(`{"schema_version":1,"id":"io-shared-cleanup","phase":"done","cleanup_attempt":{"operation":%q,"token":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","started_at":"2026-09-29T00:00:00Z"}}`, operation))
			record, err := Decode("io-shared-cleanup", raw)
			if err != nil {
				t.Fatalf("valid operation rejected: %v", err)
			}
			lease, err := DecodeLease("io-shared-cleanup", raw)
			if err != nil {
				t.Fatal(err)
			}
			a, err := Encode(record)
			if err != nil {
				t.Fatal(err)
			}
			b, err := EncodeLease(lease)
			if err != nil {
				t.Fatal(err)
			}
			for _, got := range [][]byte{a, b} {
				var decoded map[string]any
				if err := json.Unmarshal(got, &decoded); err != nil {
					t.Fatal(err)
				}
				attempt, ok := decoded["cleanup_attempt"].(map[string]any)
				if !ok || attempt["operation"] != operation {
					t.Fatalf("operation lost: %s", got)
				}
			}
		})
	}
}

func TestCleanupAttemptRejectsRetiredDraftField(t *testing.T) {
	raw := []byte(`{"schema_version":1,"id":"io-draft-attempt","phase":"done","cleanup_finish_attempt":{"token":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","started_at":"2026-09-29T00:00:00Z"}}`)
	if _, err := Decode("io-draft-attempt", raw); err == nil {
		t.Error("retired draft field accepted by cycle codec")
	}
	if _, err := DecodeLease("io-draft-attempt", raw); err == nil {
		t.Error("retired draft field accepted by lease codec")
	}
}

func TestCleanupOperationCodecRejectsMissingAndUnknownOperation(t *testing.T) {
	for _, operation := range []string{"", `"operation":"",`, `"operation":"unknown",`, `"operation":null,`} {
		raw := []byte(`{"schema_version":1,"id":"io-operation-invalid","phase":"done","cleanup_attempt":{` + operation + `"token":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","started_at":"2026-09-29T00:00:00Z"}}`)
		if _, err := Decode("io-operation-invalid", raw); err == nil {
			t.Errorf("cycle accepted %q", operation)
		}
		if _, err := DecodeLease("io-operation-invalid", raw); err == nil {
			t.Errorf("lease accepted %q", operation)
		}
	}
}
