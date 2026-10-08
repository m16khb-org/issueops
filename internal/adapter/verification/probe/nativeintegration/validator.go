package nativeintegration

import (
	selfverify "issueops/internal/contract/selfverify"
)

// Validator binds the installed host surfaces for one verification run.
type Validator struct {
	ListSkillNames             func(string) ([]string, error)
	SkillNamesForHost          func(string, []string, string) ([]string, []string)
	ResolveStableNativeRoot    func(string) (string, error)
	CodexHooksConfig           func(string) map[string]any
	OmoLifecycleExtension      func(string) string
	OmpLifecycleExtension      func(string) string
	VerifyHookConfigActivation func(map[string]any, map[string]any) (string, error)
}

func (v Validator) Validate(root string) selfverify.StepResult {
	return validateNativeIntegrationWithDeps(root, nativeIntegrationValidationDeps{Validator: v})
}
