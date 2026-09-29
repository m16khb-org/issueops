package nativeintegration

import (
	codexadapter "issueops/internal/adapter/codex"
	"issueops/internal/adapter/hostprotocol"
	installadapter "issueops/internal/adapter/install"
	installutiladapter "issueops/internal/adapter/installutil"
)

func init() {
	ResolveStableNativeRoot = installadapter.ResolveStableNativeRoot
	CodexHooksConfig = codexadapter.HooksConfig
	OmoLifecycleExtension = hostprotocol.OmoLifecycleExtension
	VerifyHookConfigActivation = installutiladapter.VerifyHookConfigActivation
}
