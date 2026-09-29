package installcli

import (
	installapp "issueops/internal/application/install"
	"issueops/internal/port"
)

// installTransactionEffects connects the application workflow to host files and CLIs.
type installTransactionEffects struct{ command Command }

func (effects installTransactionEffects) PreparePath(result *port.NativeInstallResult, request port.NativeInstallRequest, candidate, mode string) (installapp.PathTransaction, error) {
	return effects.command.prepareInstallPathPlanForCandidate(result, request, candidate, mode)
}

func (effects installTransactionEffects) Install(request port.NativeInstallRequest) (port.NativeInstallResult, error) {
	return effects.command.InstallNative(request)
}

func (effects installTransactionEffects) PlanShell(result *port.NativeInstallResult, request port.NativeInstallRequest, mode string) error {
	return planShellPath(result, request, mode)
}

func (effects installTransactionEffects) PrepareHost(plan port.NativeInstallResult) (installapp.HostTransaction, error) {
	return prepareInstallHostTransaction(plan)
}

func (effects installTransactionEffects) AppendUpstream(result *port.NativeInstallResult, root string, dryRun bool) {
	effects.command.appendUpstreamMessages(result, root, dryRun)
}
