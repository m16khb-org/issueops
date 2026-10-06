package updatecli

import (
	"fmt"
	adapter "issueops/internal/adapter/update"
	app "issueops/internal/application/update"
	contract "issueops/internal/contract/update"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type mcpProxyProcess = contract.MCPProxyProcess
type MCPCleanupProcess = contract.MCPCleanupProcess
type Deps struct{ IssueOpsRoot func() string }

var deps = Deps{IssueOpsRoot: func() string {
	root := os.Getenv("ISSUEOPS_ROOT")
	if root != "" {
		return root
	}
	cwd, _ := os.Getwd()
	return cwd
}}

func Configure(d Deps) { deps = d }
func Reset() {
	deps = Deps{IssueOpsRoot: func() string {
		root := os.Getenv("ISSUEOPS_ROOT")
		if root != "" {
			return root
		}
		cwd, _ := os.Getwd()
		return cwd
	}}
}
func testRuntime() adapter.Runtime {
	ps, err := exec.LookPath("ps")
	return adapter.Runtime{Root: deps.IssueOpsRoot(), Environment: os.Environ(), Input: os.Stdin, Output: os.Stdout, Diagnostics: os.Stderr, ProcessTable: adapter.ProcessTable{Path: ps, PathError: err, Environment: os.Environ()}}
}

var installScriptCommandRunner = func(script string, args ...string) error {
	return testRuntime().Install(filepath.Dir(filepath.Dir(script)), args)
}
var mcpProxyProcessLister = func() ([]mcpProxyProcess, error) { return testRuntime().List() }
var mcpProxyTerminator = func(pid int) error { return testRuntime().Terminate(pid) }
var mcpProxyOrphanTerminationSupported = func() bool { return runtime.GOOS == "darwin" }

type testInstaller struct{}

func (testInstaller) Install(root string, args []string) error {
	script := filepath.Join(root, "scripts", "install-native.sh")
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("install script not found at %s: %w", script, err)
	}
	return installScriptCommandRunner(script, args...)
}
func testCommand() Command {
	return Command{Root: deps.IssueOpsRoot(), Service: app.Service{Installer: testInstaller{}}}
}
func runInstallScriptCommand(name string, args []string) error { return testCommand().Run(name, args) }
func runUpdate(args []string) error                            { return runInstallScriptCommand("update", args) }
func runBootstrap(args []string) error                         { return runInstallScriptCommand("bootstrap", args) }

type testMCPProxyEffects struct{}

func (testMCPProxyEffects) List() ([]contract.MCPProxyProcess, error) { return mcpProxyProcessLister() }
func (testMCPProxyEffects) Terminate(pid int) error                   { return mcpProxyTerminator(pid) }
func (testMCPProxyEffects) CurrentPID() int                           { return os.Getpid() }
func (testMCPProxyEffects) SupportsOrphanTermination() bool {
	return mcpProxyOrphanTerminationSupported()
}
func CleanupMCPProxies(dry bool) (contract.MCPCleanupResult, error) {
	return app.CleanupMCPProxies(testMCPProxyEffects{}, dry)
}
func parseMCPProxyProcessSnapshot(line, binary string) (mcpProxyProcess, bool) {
	return adapter.ParseMCPProxyProcessSnapshot(line, binary)
}
