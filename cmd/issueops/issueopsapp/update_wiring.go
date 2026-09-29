package issueopsapp

import (
	"issueops/cmd/issueops/updatecli"
	daemonadapter "issueops/internal/adapter/daemon"
	adapter "issueops/internal/adapter/update"
	app "issueops/internal/application/update"
	"os"
	"os/exec"
)

func newUpdateRuntime() adapter.Runtime {
	env := os.Environ()
	ps, err := exec.LookPath("ps")
	return adapter.Runtime{Root: issueOpsRoot(), Environment: env, Input: os.Stdin, Output: os.Stdout, Diagnostics: os.Stderr, ProcessTable: adapter.ProcessTable{Path: ps, PathError: err, Environment: env}, InspectProcess: (daemonadapter.ProcessInspector{PSExecutable: ps, PSLookupError: err, Environment: env}).Inspect}
}
func newUpdateCommand() updatecli.Command {
	runtime := newUpdateRuntime()
	refresh := app.DaemonRefresh{Stop: runtime.StopDaemon, Cleanup: app.StaleDaemons{List: runtime.Daemons, Terminate: runtime.Terminate, CurrentPID: runtime.CurrentPID}}
	return updatecli.Command{Root: runtime.Root, Service: app.Service{Installer: runtime, RefreshDaemon: refresh.Run}}
}
func newMCPCleanupCommand() updatecli.CleanupCommand {
	return updatecli.CleanupCommand{Effects: newUpdateRuntime()}
}
func runUpdate(args []string) error    { return newUpdateCommand().Run("update", args) }
func runBootstrap(args []string) error { return newUpdateCommand().Run("bootstrap", args) }
