package probe

import (
	"issueops/internal/adapter/codex"
	"issueops/internal/adapter/hostprotocol"
	"issueops/internal/adapter/install"
	"issueops/internal/adapter/installutil"
	"issueops/internal/adapter/verification/probe/nativeintegration"
)

func ValidateNativeIntegration(root string) StepResult {
	v := nativeintegration.Validator{ListSkillNames: install.ListSkillNames, SkillNamesForHost: installutil.SkillNamesForHost, ResolveStableNativeRoot: install.ResolveStableNativeRoot, CodexHooksConfig: codex.HooksConfig, OmoLifecycleExtension: hostprotocol.OmoLifecycleExtension, VerifyHookConfigActivation: installutil.VerifyHookConfigActivation}
	return v.Validate(root)
}
