package selfverify

import (
	"errors"
	"reflect"
	"testing"

	selfaugmentcontract "issueops/internal/contract/selfaugment"
	selfverifycontract "issueops/internal/contract/selfverify"
)

func TestRunKeepsVerificationFailurePrimaryAndStillSaves(t *testing.T) {
	wantErr := errors.New("verification failed")
	order := []string{}
	err := Run(RunRequest{Loop: LoopRequest{BaseSeed: 7, TargetScore: 95}, LLMEnabled: true, SaveState: true, StateKey: "run", JSONOutput: true}, RunDeps{
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
		PrintJSON: func(any) error { order = append(order, "print"); return nil },
	})
	if !errors.Is(err, wantErr) || !reflect.DeepEqual(order, []string{"verify", "save", "print"}) {
		t.Fatalf("err=%v order=%v", err, order)
	}
}

func TestRunAppliesLLMBeforeSavingAndReturnsSaveFailure(t *testing.T) {
	wantErr := errors.New("save failed")
	order := []string{}
	err := Run(RunRequest{Loop: LoopRequest{TargetScore: 95}, LLMEnabled: true, LLMMode: "gate", SaveState: true}, RunDeps{
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
