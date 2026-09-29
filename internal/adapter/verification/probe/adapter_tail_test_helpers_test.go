package probe

import (
	codexadapter "issueops/internal/adapter/codex"
	"issueops/internal/adapter/hostprotocol"
	installadapter "issueops/internal/adapter/install"
	installutiladapter "issueops/internal/adapter/installutil"
	nativeintegrationt4d "issueops/internal/adapter/verification/probe/nativeintegration"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	nativeintegrationt4d.SkillNamesForHost = installutiladapter.SkillNamesForHost
	nativeintegrationt4d.ResolveStableNativeRoot = installadapter.ResolveStableNativeRoot
	nativeintegrationt4d.CodexHooksConfig = codexadapter.HooksConfig
	nativeintegrationt4d.OmoLifecycleExtension = hostprotocol.OmoLifecycleExtension
	nativeintegrationt4d.VerifyHookConfigActivation = installutiladapter.VerifyHookConfigActivation
}
