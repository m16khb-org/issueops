package selfworkflow

import (
	selfverify "issueops/internal/contract/selfverify"

	"io"

	"issueops/cmd/issueops/selfworkflow/llmeval"
	"issueops/cmd/issueops/selfworkflow/progress"
	verifyapp "issueops/internal/application/selfverify"
	augmentcontract "issueops/internal/contract/selfaugment"
	verifydomain "issueops/internal/domain/selfverify"
)

type SelfVerifyLLMEvalConfig = llmeval.SelfVerifyLLMEvalConfig
type SelfVerifyLLMEvalInput = llmeval.SelfVerifyLLMEvalInput
type SelfVerifyLLMEvalOptions = selfverify.LLMEvalOptions
type SelfVerifyLLMEvalResult = augmentcontract.SelfVerifyLLMEvalResult
type SelfVerifyPlannedStep = verifyapp.SelfVerifyPlannedStep
type SelfVerifyProgressEvent = selfverify.ProgressEvent
type SelfVerifyProgressReporter = progress.SelfVerifyProgressReporter
type SelfVerifyStepDeps = verifyapp.SelfVerifyStepDeps
type SelfVerifyRiskQAEvidence = verifyapp.RiskQAEvidence

func ValidateSelfVerifyLLMEvalMode(mode string) error {
	return llmeval.ValidateSelfVerifyLLMEvalMode(mode)
}

func NormalizeSelfVerifyLLMEvalMode(mode string) string {
	return llmeval.NormalizeSelfVerifyLLMEvalMode(mode)
}

func ResolveSelfVerifyLLMEvalConfig(llmEvalFlagSet bool, llmEvalFlagValue bool, llmEvalMode string, llmEvalModeFlagSet bool, lookupEnv func(string) (string, bool)) (SelfVerifyLLMEvalConfig, error) {
	return llmeval.ResolveSelfVerifyLLMEvalConfig(llmEvalFlagSet, llmEvalFlagValue, llmEvalMode, llmEvalModeFlagSet, lookupEnv)
}

func ParseSelfVerifyLLMEvalEnv(value string) (bool, string, error) {
	return llmeval.ParseSelfVerifyLLMEvalEnv(value)
}

func BoundedLLMEvalError(prefix string, err error, output string) string {
	return llmeval.BoundedLLMEvalError(prefix, err, output)
}

func ApplySelfVerifyLLMEval(result SelfAugmentResult, opts SelfVerifyLLMEvalOptions) (SelfAugmentResult, error) {
	return llmeval.ApplySelfVerifyLLMEval(result, opts)
}

func SelfVerifyLLMResponseSchemaExample() string {
	return llmeval.SelfVerifyLLMResponseSchemaExample()
}

func SelfVerifyLLMResponseFieldTypes() []string {
	return llmeval.SelfVerifyLLMResponseFieldTypes()
}

func NewSelfVerifyProgressReporter(mode string, writer io.Writer) (*SelfVerifyProgressReporter, error) {
	return progress.NewSelfVerifyProgressReporter(mode, writer)
}

func selfVerifyRerunCommands(failedStep string, baseSeed int64, targetScore float64) []string {
	return verifydomain.SelfVerifyRerunCommands(failedStep, baseSeed, targetScore)
}

func selfVerifyStepRerunCommand(label string) (string, bool) {
	return verifydomain.SelfVerifyStepRerunCommand(label)
}

func formatScore(score float64) string {
	return verifydomain.FormatScore(score)
}

func boolPtr(value bool) *bool {
	return &value
}
