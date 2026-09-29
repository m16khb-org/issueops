package issueopsapp

import (
	codext4deps "issueops/internal/adapter/codex"
	"issueops/internal/adapter/hostprotocol"

	installadapter "issueops/internal/adapter/install"
	installutiladapter "issueops/internal/adapter/installutil"

	nativeintegrationt4deps "issueops/internal/adapter/verification/probe/nativeintegration"
)

// configureAdapterTail은 설치 계획 수립과 프로젝트 문서 관측을 설치한다.
func configureAdapterTail() {
	nativeintegrationt4deps.SkillNamesForHost = installutiladapter.SkillNamesForHost
	nativeintegrationt4deps.ResolveStableNativeRoot = installadapter.ResolveStableNativeRoot
	nativeintegrationt4deps.CodexHooksConfig = codext4deps.HooksConfig
	nativeintegrationt4deps.OmoLifecycleExtension = hostprotocol.OmoLifecycleExtension
	nativeintegrationt4deps.VerifyHookConfigActivation = installutiladapter.VerifyHookConfigActivation
}
