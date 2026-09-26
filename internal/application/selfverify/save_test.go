package selfverify

import (
	"errors"
	"testing"
	"time"

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
