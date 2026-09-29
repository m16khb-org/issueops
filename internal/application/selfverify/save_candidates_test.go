package selfverify

import (
	"errors"
	"reflect"
	"testing"
	"time"

	contract "issueops/internal/contract/selfaugment"
	state "issueops/internal/contract/state"
)

func TestSaveCandidateExportFailureBoundaries(t *testing.T) {
	for _, stage := range []string{"encode", "write"} {
		t.Run(stage, func(t *testing.T) {
			failure := errors.New(stage + " failed")
			result := contract.SelfVerificationCandidateExportResult{OK: true}
			var calls []string
			err := SaveCandidateExport(&result, "", SaveCandidateExportDeps{
				Now: func() time.Time { calls = append(calls, "clock"); return time.Unix(10, 0) },
				Encode: func(snapshot contract.SelfVerificationCandidateExportStateSnapshot) ([]byte, error) {
					calls = append(calls, "encode")
					if snapshot.SchemaVersion != 1 || snapshot.Kind != contract.SelfVerificationCandidateExportKind || snapshot.GeneratedAt != "1970-01-01T00:00:10Z" {
						t.Fatalf("snapshot=%+v", snapshot)
					}
					if stage == "encode" {
						return nil, failure
					}
					return []byte("serialized"), nil
				},
				Write: func(key, body string) (state.StateResult, error) {
					calls = append(calls, "write")
					if key != "self-verify-candidates-latest" || body != "serialized" {
						t.Fatalf("write(%q,%q)", key, body)
					}
					return state.StateResult{}, failure
				},
				StateDir: func() string { calls = append(calls, "state-dir"); return "/state" },
			})
			wantCalls := []string{"clock", "encode"}
			wantDir := ""
			if stage == "write" {
				wantCalls = append(wantCalls, "write", "state-dir")
				wantDir = "/state"
			}
			checkpoint := result.StateCheckpoint
			if !errors.Is(err, failure) || !reflect.DeepEqual(calls, wantCalls) || checkpoint == nil || checkpoint.OK || checkpoint.Key != "self-verify-candidates-latest" || checkpoint.StateDir != wantDir || checkpoint.Error != failure.Error() || !result.OK {
				t.Fatalf("calls=%v checkpoint=%+v result.OK=%v err=%v", calls, checkpoint, result.OK, err)
			}
		})
	}
}

func TestSaveCandidateExportPreservesProjectionAndWriteReceipt(t *testing.T) {
	at := time.Date(2026, 9, 29, 12, 34, 56, 123, time.FixedZone("KST", 9*60*60))
	result := contract.SelfVerificationCandidateExportResult{
		OK: true, LoopKind: "self_verification", KoreanName: "검증", IssueOpsRoot: "/repo",
		GeneratedAt: "old timestamp", SourcePath: "/repo/catalog", CandidateCount: 7,
		OpenCandidateIDs: []string{"b", "a"}, SatisfiedCandidateIDs: []string{},
	}
	writes := 0
	err := SaveCandidateExport(&result, " custom ", SaveCandidateExportDeps{
		Now: func() time.Time { return at },
		Encode: func(got contract.SelfVerificationCandidateExportStateSnapshot) ([]byte, error) {
			if got.GeneratedAt != "2026-09-29T03:34:56.000000123Z" || got.CandidateCount != 7 || !reflect.DeepEqual(got.OpenCandidateIDs, []string{"b", "a"}) || got.SatisfiedCandidateIDs == nil || got.Candidates != nil || got.IssueOpsRoot != "/repo" || got.SourcePath != "/repo/catalog" || got.LoopKind != result.LoopKind || got.KoreanName != result.KoreanName || !got.OK {
				t.Fatalf("snapshot=%+v", got)
			}
			return []byte("encoded"), nil
		},
		Write: func(key, body string) (state.StateResult, error) {
			writes++
			if key != " custom " || body != "encoded" {
				t.Fatalf("write(%q,%q)", key, body)
			}
			return state.StateResult{OK: true, StateDir: "/actual", Path: "/actual/db", Record: state.RecordEnvelope{Key: "custom", Bytes: 7}}, nil
		},
		StateDir: func() string { t.Fatal("successful write must use receipt"); return "" },
	})
	want := &contract.SelfAugmentStateCheckpoint{OK: true, Key: "custom", StateDir: "/actual", Path: "/actual/db", Bytes: 7}
	if err != nil || writes != 1 || !reflect.DeepEqual(result.StateCheckpoint, want) || result.GeneratedAt != "old timestamp" {
		t.Fatalf("result=%+v writes=%d err=%v", result, writes, err)
	}
}
