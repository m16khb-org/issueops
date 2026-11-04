// Package llmeval adapts the self-verify LLM evaluation gate for the CLI layer:
// it resolves the gate configuration from flags and environment, builds the
// bounded evaluation prompt, and trims evaluator errors before they are reported.
package llmeval

import (
	selfverify "issueops/internal/contract/selfverify"

	app "issueops/internal/application/selfverify"
	contract "issueops/internal/contract/selfaugment"
)

// ApplySelfVerifyLLMEval runs the opt-in LLM evaluation gate on a self-verify result,
// wiring the llmeval prompt builder and error bounder into the application use case.
func ApplySelfVerifyLLMEval(result contract.Result, opts selfverify.LLMEvalOptions) (contract.Result, error) {
	return app.ApplyLLMEval(result, opts, app.LLMEvalDeps{BuildPrompt: BuildSelfVerifyLLMEvalPrompt, BoundError: BoundedLLMEvalError})
}
