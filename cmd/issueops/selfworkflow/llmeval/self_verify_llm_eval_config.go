package llmeval

import (
	"os"

	selfverifydomain "issueops/internal/domain/selfverify"
)

const EnvName = selfverifydomain.LLMEvalEnvName

type SelfVerifyLLMEvalConfig struct {
	Enabled bool
	Mode    string
}

func ValidateSelfVerifyLLMEvalMode(mode string) error {
	return selfverifydomain.ValidateLLMEvalMode(mode)
}

func NormalizeSelfVerifyLLMEvalMode(mode string) string {
	return selfverifydomain.NormalizeLLMEvalMode(mode)
}

func ResolveSelfVerifyLLMEvalConfig(llmEvalFlagSet bool, llmEvalFlagValue bool, llmEvalMode string, llmEvalModeFlagSet bool, lookupEnv func(string) (string, bool)) (SelfVerifyLLMEvalConfig, error) {
	config := SelfVerifyLLMEvalConfig{Mode: "advisory"}
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	ignoreEnv := llmEvalFlagSet && !llmEvalFlagValue
	if value, ok := lookupEnv(EnvName); ok && !ignoreEnv {
		enabled, mode, err := ParseSelfVerifyLLMEvalEnv(value)
		if err != nil {
			return config, err
		}
		config.Enabled = enabled
		config.Mode = mode
	}
	if llmEvalModeFlagSet {
		mode := NormalizeSelfVerifyLLMEvalMode(llmEvalMode)
		if err := ValidateSelfVerifyLLMEvalMode(mode); err != nil {
			return config, err
		}
		config.Mode = mode
	}
	if llmEvalFlagSet {
		config.Enabled = llmEvalFlagValue
	}
	return config, nil
}

func ParseSelfVerifyLLMEvalEnv(value string) (bool, string, error) {
	return selfverifydomain.ParseLLMEvalEnv(value)
}
