package selfverify

import (
	augment "issueops/internal/contract/selfaugment"
	verify "issueops/internal/contract/selfverify"
	augmentdomain "issueops/internal/domain/selfaugment"
	domain "issueops/internal/domain/selfverify"
)

type LLMEvalDeps struct {
	BuildPrompt func(augment.SelfAugmentResult) (string, int, error)
	BoundError  func(string, error, string) string
}

func ApplyLLMEval(result augment.SelfAugmentResult, opts verify.LLMEvalOptions, deps LLMEvalDeps) (augment.SelfAugmentResult, error) {
	if !opts.Enabled {
		return result, nil
	}
	mode := domain.NormalizeLLMEvalMode(opts.Mode)
	if err := domain.ValidateLLMEvalMode(mode); err != nil {
		return result, err
	}
	evidencePacket, evidenceBytes, err := deps.BuildPrompt(result)
	if err != nil {
		result.LLMEval = &augment.SelfVerifyLLMEvalResult{
			OK:                  false,
			Mode:                mode,
			ExecutionClass:      "foreground_blocking",
			ReadOnly:            true,
			EvidencePacketBytes: evidenceBytes,
			Error:               deps.BoundError("build LLM evidence packet", err, ""),
		}
		return augmentdomain.ApplySelfVerifyLLMGate(result, opts.TargetScore)
	}

	eval := augment.SelfVerifyLLMEvalResult{
		Mode:                mode,
		ExecutionClass:      "foreground_blocking",
		ReadOnly:            true,
		EvidencePacketBytes: evidenceBytes,
		Prompt:              evidencePacket,
		Error:               "self-verify external LLM evaluation was removed; run the rendered prompt with the host agent and record the result file",
	}
	result.LLMEval = &eval
	return augmentdomain.ApplySelfVerifyLLMGate(result, opts.TargetScore)
}
