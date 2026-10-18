package nativeintegration

import (
	"issueops/internal/adapter/codex"
	"issueops/internal/adapter/hostprotocol"
	"issueops/internal/adapter/install"
	"issueops/internal/adapter/installutil"
	"issueops/internal/domain/nativehost"
)

func testNativeValidator() Validator {
	return Validator{ListSkillNames: install.ListSkillNames, SkillNamesForHost: installutil.SkillNamesForHost, ResolveStableNativeRoot: install.ResolveStableNativeRoot, CodexHooksConfig: codex.HooksConfig, OmoLifecycleExtension: hostprotocol.OmoLifecycleExtension, OmpLifecycleExtension: hostprotocol.OmpLifecycleExtension, OpencodeLifecyclePlugin: hostprotocol.OpencodeLifecyclePlugin, OpencodeConfigRoot: func(home string) string { return nativehost.OpencodeConfigRoot(home, "") }, VerifyHookConfigActivation: installutil.VerifyHookConfigActivation}
}
