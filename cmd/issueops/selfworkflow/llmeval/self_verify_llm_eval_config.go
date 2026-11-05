package llmeval

import (
	"os"

	selfverifydomain "issueops/internal/domain/selfverify"
)

// SelfVerifyLLMEvalConfig is the resolved LLM evaluation gate setting:
// whether the gate runs and in which mode (for example "advisory").
type SelfVerifyLLMEvalConfig struct {
	Enabled bool
	Mode    string
}

// ResolveSelfVerifyLLMEvalConfig merges the environment variable and the CLI flags into a
// SelfVerifyLLMEvalConfig. Flags override the environment; an explicit disabling flag ignores
// the environment entirely. A nil lookupEnv falls back to os.LookupEnv. It returns an error
// when the environment value or the mode flag is invalid.
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
