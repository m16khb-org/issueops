package llmeval

import (
	selfverify "issueops/internal/contract/selfverify"

	app "issueops/internal/application/selfverify"
	contract "issueops/internal/contract/selfaugment"
)

func ApplySelfVerifyLLMEval(result contract.SelfAugmentResult, opts selfverify.LLMEvalOptions) (contract.SelfAugmentResult, error) {
	return app.ApplyLLMEval(result, opts, app.LLMEvalDeps{BuildPrompt: BuildSelfVerifyLLMEvalPrompt, BoundError: BoundedLLMEvalError})
}
