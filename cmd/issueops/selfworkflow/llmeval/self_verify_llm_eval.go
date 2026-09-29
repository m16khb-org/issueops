package llmeval

import (
	app "issueops/internal/application/selfverify"
	contract "issueops/internal/contract/selfaugment"
)

func ApplySelfVerifyLLMEval(result contract.SelfAugmentResult, opts SelfVerifyLLMEvalOptions) (contract.SelfAugmentResult, error) {
	return app.ApplyLLMEval(result, opts, app.LLMEvalDeps{BuildPrompt: BuildSelfVerifyLLMEvalPrompt, BoundError: BoundedLLMEvalError})
}
