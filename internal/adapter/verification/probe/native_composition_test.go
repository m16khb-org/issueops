package probe

import (
	selfverify "issueops/internal/contract/selfverify"

	"issueops/internal/adapter/codex"
	"issueops/internal/adapter/hostprotocol"
	"issueops/internal/adapter/install"
	"issueops/internal/adapter/installutil"
	"issueops/internal/adapter/verification/probe/nativeintegration"
	"issueops/internal/domain/nativehost"
)

func ValidateNativeIntegration(root string) selfverify.StepResult {
	v := nativeintegration.Validator{ListSkillNames: install.ListSkillNames, SkillNamesForHost: installutil.SkillNamesForHost, ResolveStableNativeRoot: install.ResolveStableNativeRoot, CodexHooksConfig: codex.HooksConfig, OmoLifecycleExtension: hostprotocol.OmoLifecycleExtension, OmpLifecycleExtension: hostprotocol.OmpLifecycleExtension, OpencodeLifecyclePlugin: hostprotocol.OpencodeLifecyclePlugin, OpencodeConfigRoot: func(home string) string { return nativehost.OpencodeConfigRoot(home, "") }, VerifyHookConfigActivation: installutil.VerifyHookConfigActivation}
	return v.Validate(root)
}
