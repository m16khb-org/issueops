package nativeintegration

import (
	"issueops/internal/adapter/codex"
	"issueops/internal/adapter/hostprotocol"
	"issueops/internal/adapter/install"
	"issueops/internal/adapter/installutil"
)

func testNativeValidator() Validator {
	return Validator{ListSkillNames: install.ListSkillNames, SkillNamesForHost: installutil.SkillNamesForHost, ResolveStableNativeRoot: install.ResolveStableNativeRoot, CodexHooksConfig: codex.HooksConfig, OmoLifecycleExtension: hostprotocol.OmoLifecycleExtension, OmpLifecycleExtension: hostprotocol.OmpLifecycleExtension, VerifyHookConfigActivation: installutil.VerifyHookConfigActivation}
}
