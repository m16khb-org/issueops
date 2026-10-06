package issueopsapp

import (
	"issueops/cmd/issueops/updatecli"
	"issueops/internal/adapter/processinspect"
	adapter "issueops/internal/adapter/update"
	app "issueops/internal/application/update"
	"os"
	"os/exec"
)

func newUpdateRuntime() adapter.Runtime {
	env := os.Environ()
	ps, err := exec.LookPath("ps")
	return adapter.Runtime{Root: issueOpsRoot(), Environment: env, Input: os.Stdin, Output: os.Stdout, Diagnostics: os.Stderr, ProcessTable: adapter.ProcessTable{Path: ps, PathError: err, Environment: env}, InspectProcess: (processinspect.Inspector{PSExecutable: ps, PSLookupError: err, Environment: env}).Inspect}
}
func newUpdateCommand() updatecli.Command {
	runtime := newUpdateRuntime()
	return updatecli.Command{Root: runtime.Root, Service: app.Service{Installer: runtime}}
}
func newMCPCleanupCommand() updatecli.CleanupCommand {
	return updatecli.CleanupCommand{Effects: newUpdateRuntime()}
}
func runUpdate(args []string) error    { return newUpdateCommand().Run("update", args) }
func runBootstrap(args []string) error { return newUpdateCommand().Run("bootstrap", args) }
