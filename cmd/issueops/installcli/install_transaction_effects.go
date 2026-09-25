package installcli

import (
	installapp "issueops/internal/application/install"
	"issueops/internal/port"
)

// installTransactionEffects connects the application workflow to host files and CLIs.
type installTransactionEffects struct{}

func (installTransactionEffects) PreparePath(result *port.NativeInstallResult, request port.NativeInstallRequest, candidate, mode string) (installapp.PathTransaction, error) {
	return prepareInstallPathPlanForCandidate(result, request, candidate, mode)
}

func (installTransactionEffects) Install(request port.NativeInstallRequest) (port.NativeInstallResult, error) {
	return deps.InstallNative(request)
}

func (installTransactionEffects) PlanShell(result *port.NativeInstallResult, request port.NativeInstallRequest, mode string) error {
	return planShellPath(result, request, mode)
}

func (installTransactionEffects) PrepareHost(plan port.NativeInstallResult) (installapp.HostTransaction, error) {
	return prepareInstallHostTransaction(plan)
}

func (installTransactionEffects) AppendUpstream(result *port.NativeInstallResult, root string, dryRun bool) {
	appendUpstreamMessages(result, root, dryRun)
}
