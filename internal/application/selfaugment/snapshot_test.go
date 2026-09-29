package selfaugment

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	contract "issueops/internal/contract/selfaugment"
	state "issueops/internal/contract/state"
)

func TestSnapshotStoreRejectsReadAndDecodeFailures(t *testing.T) {
	fault := errors.New("read failed")
	for _, tc := range []struct {
		content string
		err     error
		want    string
	}{
		{"", fault, "read failed"},
		{"{", nil, "unexpected end of JSON input"},
		{`{"schema_version":1,"kind":"retired"}`, nil, "contains kind"},
		{`{"kind":"self_verification_summary"}`, nil, "schema 0"},
	} {
		store := SnapshotStore{ReadState: func(key string) (state.StateResult, error) {
			if key != "summary" {
				t.Fatalf("key=%q", key)
			}
			return state.StateResult{Record: state.RecordEnvelope{Content: tc.content}}, tc.err
		}}
		got, err := store.Read("summary")
		if err == nil || !strings.Contains(err.Error(), tc.want) || got.Kind != "" {
			t.Fatalf("snapshot=%+v err=%v", got, err)
		}
	}
}

func TestSnapshotStoreWritesOnlyValidEnvelopeAndPreservesErrors(t *testing.T) {
	for _, failAt := range []string{"key", "encode", "write", ""} {
		t.Run(failAt, func(t *testing.T) {
			fault := errors.New("unavailable")
			clockCalls, writeCalls := 0, 0
			snapshot := contract.SelfAugmentStateSnapshot{SchemaVersion: 1, Kind: "self_verification_summary"}
			if failAt == "encode" {
				snapshot.TargetScore = math.NaN()
			}
			store := SnapshotStore{
				NormalizeKey: func(key string) (string, error) {
					if key != " key " {
						t.Fatalf("key=%q", key)
					}
					if failAt == "key" {
						return "", fault
					}
					return "key", nil
				},
				Now: func() time.Time {
					clockCalls++
					return time.Date(2026, 1, 2, 9, 0, 0, 0, time.FixedZone("KST", 9*60*60))
				},
				WriteRecord: func(dir, key string, record state.RecordEnvelope) (string, error) {
					writeCalls++
					if dir != "/state" || key != "key" || record.SchemaVersion != state.SchemaVersion || record.Key != "key" || record.UpdatedAt != "2026-01-02T00:00:00Z" || record.Bytes != len(record.Content) || !strings.Contains(record.Content, `"failure_cause": "none"`) {
						t.Fatalf("dir=%s key=%s record=%+v", dir, key, record)
					}
					if failAt == "write" {
						return "", fault
					}
					return "/state/key", nil
				},
			}
			err := store.Write("/state", " key ", snapshot)
			wantCalls := 1
			if failAt == "key" || failAt == "encode" {
				wantCalls = 0
			}
			if clockCalls != wantCalls || writeCalls != wantCalls || (err == nil) != (failAt == "") {
				t.Fatalf("clock=%d write=%d err=%v", clockCalls, writeCalls, err)
			}
			if (failAt == "key" || failAt == "write") && !errors.Is(err, fault) {
				t.Fatalf("lost error: %v", err)
			}
		})
	}
}
