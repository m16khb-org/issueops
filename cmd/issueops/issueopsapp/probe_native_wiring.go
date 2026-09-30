package issueopsapp

import (
	"issueops/internal/adapter/codex"
	"issueops/internal/adapter/hostprotocol"
	"issueops/internal/adapter/install"
	"issueops/internal/adapter/installutil"
	"issueops/internal/adapter/verification/probe/nativeintegration"
)

func newNativeIntegrationProbe() nativeintegration.Validator {
	return nativeintegration.Validator{ListSkillNames: install.ListSkillNames, SkillNamesForHost: installutil.SkillNamesForHost, ResolveStableNativeRoot: install.ResolveStableNativeRoot, CodexHooksConfig: codex.HooksConfig, OmoLifecycleExtension: hostprotocol.OmoLifecycleExtension, VerifyHookConfigActivation: installutil.VerifyHookConfigActivation}
}
