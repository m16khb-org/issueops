package llmeval

import (
	"os"

	selfverifydomain "issueops/internal/domain/selfverify"
)

type SelfVerifyLLMEvalConfig struct {
	Enabled bool
	Mode    string
}

func ResolveSelfVerifyLLMEvalConfig(llmEvalFlagSet bool, llmEvalFlagValue bool, llmEvalMode string, llmEvalModeFlagSet bool, lookupEnv func(string) (string, bool)) (SelfVerifyLLMEvalConfig, error) {
	config := SelfVerifyLLMEvalConfig{Mode: "advisory"}
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	ignoreEnv := llmEvalFlagSet && !llmEvalFlagValue
	if value, ok := lookupEnv(selfverifydomain.LLMEvalEnvName); ok && !ignoreEnv {
		enabled, mode, err := selfverifydomain.ParseLLMEvalEnv(value)
		if err != nil {
			return config, err
		}
		config.Enabled = enabled
		config.Mode = mode
	}
	if llmEvalModeFlagSet {
		mode := selfverifydomain.NormalizeLLMEvalMode(llmEvalMode)
		if err := selfverifydomain.ValidateLLMEvalMode(mode); err != nil {
			return config, err
		}
		config.Mode = mode
	}
	if llmEvalFlagSet {
		config.Enabled = llmEvalFlagValue
	}
	return config, nil
}
