package selfverify

import (
	selfaugmentcontract "issueops/internal/contract/selfaugment"
	selfverifycontract "issueops/internal/contract/selfverify"
)

type ExecuteRequest struct {
	Loop       LoopRequest
	LLMEnabled bool
	LLMMode    string
	SaveState  bool
	StateKey   string
}

type ExecuteDeps struct {
	Verify       func(LoopRequest) (selfaugmentcontract.SelfAugmentResult, error)
	ApplyLLMEval func(selfaugmentcontract.SelfAugmentResult, selfverifycontract.LLMEvalOptions) (selfaugmentcontract.SelfAugmentResult, error)
	SaveSummary  func(*selfaugmentcontract.SelfAugmentResult, string) error
}

func Execute(request ExecuteRequest, deps ExecuteDeps) (selfaugmentcontract.SelfAugmentResult, error) {
	result, err := deps.Verify(request.Loop)
	if err == nil && request.LLMEnabled {
		result, err = deps.ApplyLLMEval(result, selfverifycontract.LLMEvalOptions{
			Enabled: true, Mode: request.LLMMode, TargetScore: request.Loop.TargetScore,
		})
	}
	var saveErr error
	if request.SaveState {
		saveErr = deps.SaveSummary(&result, request.StateKey)
	}
	if err == nil && saveErr != nil {
		return result, saveErr
	}
	return result, err
}
