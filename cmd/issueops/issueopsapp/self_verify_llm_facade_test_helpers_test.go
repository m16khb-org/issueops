package issueopsapp

import (
	selfverify "issueops/internal/contract/selfverify"

	llmeval "issueops/cmd/issueops/selfworkflow/llmeval"
)

func applySelfVerifyLLMEval(result SelfAugmentResult, opts SelfVerifyLLMEvalOptions) (SelfAugmentResult, error) {
	return llmeval.ApplySelfVerifyLLMEval(result, opts)
}

func validateSelfVerifyLLMEvalMode(mode string) error {
	return llmeval.ValidateSelfVerifyLLMEvalMode(mode)
}

func normalizeSelfVerifyLLMEvalMode(mode string) string {
	return llmeval.NormalizeSelfVerifyLLMEvalMode(mode)
}

func resolveSelfVerifyLLMEvalConfig(llmEvalFlagSet bool, llmEvalFlagValue bool, llmEvalMode string, llmEvalModeFlagSet bool, lookupEnv func(string) (string, bool)) (SelfVerifyLLMEvalConfig, error) {
	return llmeval.ResolveSelfVerifyLLMEvalConfig(llmEvalFlagSet, llmEvalFlagValue, llmEvalMode, llmEvalModeFlagSet, lookupEnv)
}

func parseSelfVerifyLLMEvalEnv(value string) (bool, string, error) {
	return llmeval.ParseSelfVerifyLLMEvalEnv(value)
}

func boundedLLMEvalError(prefix string, err error, output string) string {
	return llmeval.BoundedLLMEvalError(prefix, err, output)
}

func buildSelfVerifyLLMEvalPrompt(result SelfAugmentResult) (string, int, error) {
	return llmeval.BuildSelfVerifyLLMEvalPrompt(result)
}

func selfVerifyLLMResponseSchemaExample() string {
	return llmeval.SelfVerifyLLMResponseSchemaExample()
}

func selfVerifyLLMResponseFieldTypes() []string {
	return llmeval.SelfVerifyLLMResponseFieldTypes()
}

type SelfVerifyLLMEvalConfig = llmeval.SelfVerifyLLMEvalConfig

type SelfVerifyLLMEvalOptions = selfverify.LLMEvalOptions
