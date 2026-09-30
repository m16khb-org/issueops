package selfverify

import (
	"fmt"
	"strings"
)

const LLMEvalEnvName = "ISSUEOPS_SELF_VERIFY_LLM_EVAL"

func NormalizeLLMEvalMode(mode string) string {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" {
		return "advisory"
	}
	return mode
}

func ValidateLLMEvalMode(mode string) error {
	switch NormalizeLLMEvalMode(mode) {
	case "advisory", "gate":
		return nil
	default:
		return fmt.Errorf("llm-eval-mode must be advisory or gate")
	}
}

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
