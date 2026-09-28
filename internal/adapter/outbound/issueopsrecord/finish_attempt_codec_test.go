package issueopsrecord

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	model "issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
	state "issueops/internal/contract/state"
)

func TestFinishAttemptCodecPreservesArmedAndDrainedRecords(t *testing.T) {
	const attempt = `{"operation":"finish","token":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","started_at":"2026-09-29T00:00:00Z"}`
	const failure = `{"step":"record_delete","message":"storage unavailable","at":"2026-09-29T00:01:00Z"}`
	for _, tc := range []struct{ name, fields string }{
		{"old schema1", ""},
		{"armed", `,"cleanup_attempt":` + attempt},
		{"undrained failure", `,"cleanup_attempt":` + attempt + `,"cleanup_finish_failure":` + failure},
		{"drained failure", `,"cleanup_finish_failure":` + failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte(`{"ok":true,"schema_version":1,"id":"io-finish-codec","repo":"/repo","phase":"done","created_at":"2026-09-29T00:00:00Z","updated_at":"2026-09-29T00:00:00Z"` + tc.fields + `}`)
			record, err := Decode("io-finish-codec", raw)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := Encode(record)
			if err != nil {
				t.Fatal(err)
			}
			lease, err := DecodeLease("io-finish-codec", raw)
			if err != nil {
				t.Fatal(err)
			}
			leaseEncoded, err := EncodeLease(lease)
			if err != nil {
				t.Fatal(err)
			}
			var original map[string]any
			if err := json.Unmarshal(raw, &original); err != nil {
				t.Fatal(err)
			}
			for _, result := range [][]byte{encoded, leaseEncoded} {
				var got map[string]any
				if err := json.Unmarshal(result, &got); err != nil {
					t.Fatal(err)
				}
				for _, field := range []string{"cleanup_attempt", "cleanup_finish_failure"} {
					wantJSON, _ := json.Marshal(original[field])
					gotJSON, _ := json.Marshal(got[field])
					if string(wantJSON) != string(gotJSON) {
						t.Fatalf("lost %s: want %s got %s", field, wantJSON, gotJSON)
					}
					_, wantPresent := original[field]
					_, gotPresent := got[field]
					if wantPresent != gotPresent {
						t.Fatalf("changed presence of %s", field)
					}
				}
			}
		})
	}
}

func TestFinishAttemptCodecRejectsMalformedAuthority(t *testing.T) {
	for _, attempt := range []string{
		`{}`, `{"operation":"finish","token":"short","started_at":"2026-09-29T00:00:00Z"}`,
		`{"operation":"finish","token":"gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg","started_at":"2026-09-29T00:00:00Z"}`,
		`{"operation":"finish","token":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}`,
		`{"operation":"finish","token":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","started_at":"yesterday"}`,
		`{"operation":"finish","token":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","started_at":"2026-09-29T00:00:00Z","unknown":true}`,
	} {
		raw := []byte(fmt.Sprintf(`{"ok":true,"schema_version":1,"id":"io-finish-codec","phase":"done","cleanup_attempt":%s}`, attempt))
		if _, err := Decode("io-finish-codec", raw); !errors.Is(err, state.ErrInvalidState) {
			t.Errorf("accepted malformed record %s: %v", attempt, err)
		}
		if _, err := DecodeLease("io-finish-codec", raw); !errors.Is(err, state.ErrInvalidState) {
			t.Errorf("accepted malformed lease record %s: %v", attempt, err)
		}
		var record model.IssueOpsRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			t.Fatal(err)
		}
		// Unknown JSON fields are checked only by strict decoding.
		if attempt == `{"operation":"finish","token":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","started_at":"2026-09-29T00:00:00Z","unknown":true}` {
			continue
		}
		if _, err := Encode(record); !errors.Is(err, state.ErrInvalidState) {
			t.Errorf("encoded malformed attempt %s: %v", attempt, err)
		}
		var lease leasecontract.Record
		if err := json.Unmarshal(raw, &lease); err != nil {
			t.Fatal(err)
		}
		if _, err := EncodeLease(lease); !errors.Is(err, state.ErrInvalidState) {
			t.Errorf("encoded malformed lease attempt %s: %v", attempt, err)
		}
	}
}
