package issueopsapp

import (
	"issueops/cmd/issueops/updatecli"
)

func runInstallScriptCommand(commandName string, args []string) error {
	resetUpdateFacadeDeps()
	return updatecli.RunInstallScriptCommand(commandName, args)
}

func terminateStaleDaemonProcesses() (int, error) {
	resetUpdateFacadeDeps()
	return updatecli.TerminateStaleDaemonProcesses()
}

func parseDaemonProcess(line, binary string) (daemonProcess, bool) {
	return updatecli.ParseDaemonProcess(line, binary)
}

func listDaemonProcesses() ([]daemonProcess, error) {
	return updatecli.ListDaemonProcesses()
}

func refreshRunningMCPProxiesAfterInstall() (int, error) {
	resetUpdateFacadeDeps()
	return updatecli.RefreshRunningMCPProxiesAfterInstall()
}

func parseMCPProxyProcess(line, binary string) (mcpProxyProcess, bool) {
	return updatecli.ParseMCPProxyProcess(line, binary)
}

func listMCPProxyProcesses() ([]mcpProxyProcess, error) {
	return updatecli.ListMCPProxyProcesses()
}
