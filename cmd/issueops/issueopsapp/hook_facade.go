package issueopsapp

import (
	"issueops/cmd/issueops/hookcli"
	"issueops/internal/adapter/hostprotocol"
)

func configureHookCLI() {
	hookcli.ResolveTarget = resolveTarget
}

func runHook(args []string) error {
	configureHookCLI()
	return hookcli.RunHook(args, hostprotocol.FormatHookContext)
}
