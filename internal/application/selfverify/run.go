package selfverify

import (
	selfaugmentcontract "issueops/internal/contract/selfaugment"
	selfverifycontract "issueops/internal/contract/selfverify"
)

type RunRequest struct {
	Loop       LoopRequest
	LLMEnabled bool
	LLMMode    string
	SaveState  bool
	StateKey   string
	JSONOutput bool
}

type RunDeps struct {
	Verify       func(LoopRequest) (selfaugmentcontract.SelfAugmentResult, error)
	ApplyLLMEval func(selfaugmentcontract.SelfAugmentResult, selfverifycontract.LLMEvalOptions) (selfaugmentcontract.SelfAugmentResult, error)
	SaveSummary  func(*selfaugmentcontract.SelfAugmentResult, string) error
	PrintJSON    func(any) error
}

func Run(request RunRequest, deps RunDeps) error {
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
	if request.JSONOutput {
		_ = deps.PrintJSON(result)
	}
	if err == nil && saveErr != nil {
		return saveErr
	}
	return err
}
