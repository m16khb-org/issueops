package selfverify

import (
	selfaugmentcontract "issueops/internal/contract/selfaugment"
	selfverifycontract "issueops/internal/contract/selfverify"
)

// ExecuteRequest configures Execute: the loop request, whether and how to run the LLM evaluation,
// and whether to save the summary under StateKey.
type ExecuteRequest struct {
	Loop       LoopRequest
	LLMEnabled bool
	LLMMode    string
	SaveState  bool
	StateKey   string
}

// ExecuteDeps injects the verification, LLM evaluation and summary saving functions that Execute
// sequences.
type ExecuteDeps struct {
	Verify       func(LoopRequest) (selfaugmentcontract.Result, error)
	ApplyLLMEval func(selfaugmentcontract.Result, selfverifycontract.LLMEvalOptions) (selfaugmentcontract.Result, error)
	SaveSummary  func(*selfaugmentcontract.Result, string) error
}

// Execute runs the verification loop, then optionally the LLM evaluation, and saves the summary
// when requested. The summary is saved even when verification failed; a save error is returned only
// when verification itself succeeded.
func Execute(request ExecuteRequest, deps ExecuteDeps) (selfaugmentcontract.Result, error) {
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
