package selfverify

import (
	"fmt"
	"strings"
)

// LLMEvalEnvName is the environment variable that enables the LLM evaluation and selects its mode.
const LLMEvalEnvName = "ISSUEOPS_SELF_VERIFY_LLM_EVAL"

// NormalizeLLMEvalMode trims and lowercases mode, defaulting an empty value to "advisory".
func NormalizeLLMEvalMode(mode string) string {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" {
		return "advisory"
	}
	return mode
}

// ValidateLLMEvalMode returns an error unless the normalized mode is advisory or gate.
func ValidateLLMEvalMode(mode string) error {
	switch NormalizeLLMEvalMode(mode) {
	case "advisory", "gate":
		return nil
	default:
		return fmt.Errorf("llm-eval-mode must be advisory or gate")
	}
}

// ParseLLMEvalEnv interprets the LLMEvalEnvName value. It returns whether the evaluation is enabled and its
// mode (advisory or gate), or an error for an unrecognized value.
func ParseLLMEvalEnv(value string) (bool, string, error) {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", "0", "false", "no", "off", "disabled":
		return false, "advisory", nil
	case "1", "true", "yes", "on", "enabled", "advisory":
		return true, "advisory", nil
	case "gate":
		return true, "gate", nil
	default:
		return false, "advisory", fmt.Errorf("%s must be off, advisory, or gate", LLMEvalEnvName)
	}
}
