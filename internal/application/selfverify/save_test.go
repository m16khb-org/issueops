package selfverify

import (
	"errors"
	"testing"
	"time"

	failurecausecontract "issueops/internal/contract/failurecause"
	selfaugmentcontract "issueops/internal/contract/selfaugment"
	statecontract "issueops/internal/contract/state"
)

func TestSaveSummaryEncodingFailureDoesNotWrite(t *testing.T) {
	wantErr := errors.New("encode failed")
	writeCalls := 0
	result := selfaugmentcontract.SelfAugmentResult{OK: true}
	err := SaveSummary(&result, "", SaveSummaryDeps{
		Now:    time.Now,
		Encode: func(selfaugmentcontract.SelfAugmentStateSnapshot) ([]byte, error) { return nil, wantErr },
		Write: func(string, string) (statecontract.StateResult, error) {
			writeCalls++
			return statecontract.StateResult{}, nil
		},
	})
	if !errors.Is(err, wantErr) || writeCalls != 0 || result.StateCheckpoint == nil || result.StateCheckpoint.OK || result.StateCheckpoint.Key != "self-verify-latest" || result.StateCheckpoint.Error != wantErr.Error() {
		t.Fatalf("result=%+v err=%v writeCalls=%d", result, err, writeCalls)
	}
}

func TestSaveSummaryWriteFailureRecordsStateDir(t *testing.T) {
	wantErr := errors.New("write failed")
	result := selfaugmentcontract.SelfAugmentResult{OK: true}
	err := SaveSummary(&result, "custom", SaveSummaryDeps{
		Now:    time.Now,
		Encode: func(selfaugmentcontract.SelfAugmentStateSnapshot) ([]byte, error) { return []byte("{}"), nil },
		Write: func(key, body string) (statecontract.StateResult, error) {
			if key != "custom" || body != "{}" {
				t.Fatalf("write key=%q body=%q", key, body)
			}
			return statecontract.StateResult{}, wantErr
		},
		StateDir: func() string { return "/state" },
	})
	if !errors.Is(err, wantErr) || result.StateCheckpoint == nil || result.StateCheckpoint.OK || result.StateCheckpoint.Key != "custom" || result.StateCheckpoint.StateDir != "/state" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestSaveSummaryClassifiesFailureCauseOnlyWhenStepsFailed(t *testing.T) {
	evidence := []failurecausecontract.Evidence{{Cause: failurecausecontract.Transport, Code: "dial_timeout", Source: "fixture"}}
	for _, tc := range []struct {
		name        string
		failedSteps int
		wantCause   failurecausecontract.Cause
		wantReason  string
		wantItems   int
	}{
		{"no failed steps", 0, failurecausecontract.None, "no_failed_steps", 0},
		{"failed step", 1, failurecausecontract.Transport, "transport:dial_timeout", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var saved selfaugmentcontract.SelfAugmentStateSnapshot
			result := selfaugmentcontract.SelfAugmentResult{Summary: selfaugmentcontract.SelfAugmentSummary{FailedSteps: tc.failedSteps, FailureCauseEvidence: evidence}}
			err := SaveSummary(&result, "k", SaveSummaryDeps{
				Now: time.Now,
				Encode: func(s selfaugmentcontract.SelfAugmentStateSnapshot) ([]byte, error) {
					saved = s
					return []byte("{}"), nil
				},
				Write: func(string, string) (statecontract.StateResult, error) { return statecontract.StateResult{}, nil },
			})
			got := saved.Summary
			if err != nil || got.FailureCause != tc.wantCause || got.FailureCauseReason != tc.wantReason || len(got.FailureCauseEvidence) != tc.wantItems {
				t.Fatalf("err=%v summary=%+v", err, got)
			}
		})
	}
}
