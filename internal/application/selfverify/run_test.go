package selfverify

import (
	"errors"
	"reflect"
	"testing"

	selfaugmentcontract "issueops/internal/contract/selfaugment"
	selfverifycontract "issueops/internal/contract/selfverify"
)

func TestExecuteKeepsVerificationFailurePrimaryAndStillSaves(t *testing.T) {
	wantErr := errors.New("verification failed")
	order := []string{}
	_, err := Execute(ExecuteRequest{Loop: LoopRequest{BaseSeed: 7, TargetScore: 95}, LLMEnabled: true, SaveState: true, StateKey: "run"}, ExecuteDeps{
		Verify: func(LoopRequest) (selfaugmentcontract.SelfAugmentResult, error) {
			order = append(order, "verify")
			return selfaugmentcontract.SelfAugmentResult{BaseSeed: 7}, wantErr
		},
		ApplyLLMEval: func(selfaugmentcontract.SelfAugmentResult, selfverifycontract.LLMEvalOptions) (selfaugmentcontract.SelfAugmentResult, error) {
			order = append(order, "llm")
			return selfaugmentcontract.SelfAugmentResult{}, nil
		},
		SaveSummary: func(result *selfaugmentcontract.SelfAugmentResult, key string) error {
			if result.BaseSeed != 7 || key != "run" {
				t.Fatalf("save result=%+v key=%q", result, key)
			}
			order = append(order, "save")
			return errors.New("save failed")
		},
	})
	if !errors.Is(err, wantErr) || !reflect.DeepEqual(order, []string{"verify", "save"}) {
		t.Fatalf("err=%v order=%v", err, order)
	}
}

func TestExecuteAppliesLLMBeforeSavingAndReturnsSaveFailure(t *testing.T) {
	wantErr := errors.New("save failed")
	order := []string{}
	_, err := Execute(ExecuteRequest{Loop: LoopRequest{TargetScore: 95}, LLMEnabled: true, LLMMode: "gate", SaveState: true}, ExecuteDeps{
		Verify: func(LoopRequest) (selfaugmentcontract.SelfAugmentResult, error) {
			order = append(order, "verify")
			return selfaugmentcontract.SelfAugmentResult{OK: true}, nil
		},
		ApplyLLMEval: func(result selfaugmentcontract.SelfAugmentResult, opts selfverifycontract.LLMEvalOptions) (selfaugmentcontract.SelfAugmentResult, error) {
			if !opts.Enabled || opts.Mode != "gate" || opts.TargetScore != 95 {
				t.Fatalf("options=%+v", opts)
			}
			order = append(order, "llm")
			result.OK = false
			return result, nil
		},
		SaveSummary: func(result *selfaugmentcontract.SelfAugmentResult, _ string) error {
			if result.OK {
				t.Fatal("saved pre-LLM result")
			}
			order = append(order, "save")
			return wantErr
		},
	})
	if !errors.Is(err, wantErr) || !reflect.DeepEqual(order, []string{"verify", "llm", "save"}) {
		t.Fatalf("err=%v order=%v", err, order)
	}
}

func TestExecutePreservesFailureAndCheckpoint(t *testing.T) {
	verifyErr := errors.New("verification failed")
	saveErr := errors.New("save failed")
	for _, tc := range []struct {
		name      string
		verifyErr error
		save      bool
		wantErr   error
	}{
		{"read-only", nil, false, nil}, {"save-failure", nil, true, saveErr}, {"verification-first", verifyErr, true, verifyErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Execute(ExecuteRequest{Loop: LoopRequest{BaseSeed: 123}, SaveState: tc.save, StateKey: "checkpoint"}, ExecuteDeps{
				Verify: func(req LoopRequest) (selfaugmentcontract.SelfAugmentResult, error) {
					return selfaugmentcontract.SelfAugmentResult{BaseSeed: req.BaseSeed}, tc.verifyErr
				},
				SaveSummary: func(result *selfaugmentcontract.SelfAugmentResult, key string) error {
					result.StateCheckpoint = &selfaugmentcontract.SelfAugmentStateCheckpoint{Key: key, OK: false, Error: saveErr.Error()}
					return saveErr
				},
			})
			if !errors.Is(err, tc.wantErr) || result.BaseSeed != 123 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if tc.save {
				if result.StateCheckpoint == nil || result.StateCheckpoint.Key != "checkpoint" || result.StateCheckpoint.Error != "save failed" {
					t.Fatalf("lost checkpoint: %+v", result)
				}
			} else if result.StateCheckpoint != nil {
				t.Fatal("unexpected save")
			}
		})
	}
}
