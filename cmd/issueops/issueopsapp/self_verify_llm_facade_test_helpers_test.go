package issueopsapp

import (
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

func decodeSelfVerifyLLMEval(out []byte, eval *SelfVerifyLLMEvalResult) error {
	return llmeval.DecodeSelfVerifyLLMEval(out, eval)
}

func decodeSelfVerifyLLMEvalStrict(out []byte, eval *SelfVerifyLLMEvalResult) error {
	return llmeval.DecodeSelfVerifyLLMEvalStrict(out, eval)
}

func extractSelfVerifyLLMEvalJSON(out []byte) ([]byte, bool) {
	return llmeval.ExtractSelfVerifyLLMEvalJSON(out)
}

func boundedLLMEvalError(prefix string, err error, output string) string {
	return llmeval.BoundedLLMEvalError(prefix, err, output)
}

func applySelfVerifyLLMGate(result SelfAugmentResult, targetScore float64) (SelfAugmentResult, error) {
	return llmeval.ApplySelfVerifyLLMGate(result, targetScore)
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

type SelfVerifyLLMEvalOptions = llmeval.SelfVerifyLLMEvalOptions
