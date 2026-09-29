package updatecli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	updateapp "issueops/internal/application/update"
	updatecontract "issueops/internal/contract/update"
)

var postInstallMCPProxyRefresh = refreshRunningMCPProxiesAfterInstall
var mcpProxyProcessLister = listMCPProxyProcesses
var mcpProxyTerminator = terminateMCPProxyProcess
var mcpProxyOrphanTerminationSupported = func() bool { return runtime.GOOS == "darwin" }

type mcpProxyProcess = updatecontract.MCPProxyProcess
type MCPCleanupProcess = updatecontract.MCPCleanupProcess
type MCPCleanupResult = updatecontract.MCPCleanupResult

func refreshRunningMCPProxiesAfterInstall() (int, error) {
	// 업데이트는 host가 소유한 stdio 수명을 보존한다. 새 daemon generation은
	// 살아 있는 proxy가 세션 초기화를 재생해 채택하며, 프로세스 정리는 명시적
	// `mcp cleanup` 경계에서만 수행한다.
	return 0, nil
}

func cleanupMCPProxies(dryRun bool) (MCPCleanupResult, error) {
	return updateapp.CleanupMCPProxies(mcpProxyCleanupEffects{}, dryRun)
}

type mcpProxyCleanupEffects struct{}

func (mcpProxyCleanupEffects) List() ([]updatecontract.MCPProxyProcess, error) {
	return mcpProxyProcessLister()
}
func (mcpProxyCleanupEffects) Terminate(pid int) error { return mcpProxyTerminator(pid) }
func (mcpProxyCleanupEffects) CurrentPID() int         { return os.Getpid() }
func (mcpProxyCleanupEffects) SupportsOrphanTermination() bool {
	return mcpProxyOrphanTerminationSupported()
}

func listMCPProxyProcesses() ([]mcpProxyProcess, error) {
	out, err := exec.Command("ps", "-axo", "pid=,ppid=,command=").Output()
	if err != nil {
		return nil, fmt.Errorf("MCP proxy inventory failed: %w", err)
	}
	binary := filepath.Join(deps.IssueOpsRoot(), "bin", "issueops")
	canonicalBinary, err := filepath.EvalSymlinks(binary)
	if err != nil {
		return nil, fmt.Errorf("resolve MCP proxy binary: %w", err)
	}
	var processes []mcpProxyProcess
	for _, line := range strings.Split(string(out), "\n") {
		process, ok := parseMCPProxyProcessSnapshot(line, canonicalBinary)
		if !ok {
			continue
		}
		identity, identityErr := deps.InspectProcess(process.PID)
		if identityErr == nil {
			process.StartTime = identity.StartTime
			process.Executable = identity.Executable
			process.IdentityVerified = identity.Executable == canonicalBinary
		}
		processes = append(processes, process)
	}
	return processes, nil
}

func parseMCPProxyProcess(line, binary string) (mcpProxyProcess, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return mcpProxyProcess{}, false
	}
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return mcpProxyProcess{}, false
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return mcpProxyProcess{}, false
	}
	command := strings.Join(fields[1:], " ")
	if command != binary+" mcp" {
		return mcpProxyProcess{}, false
	}
	return mcpProxyProcess{PID: pid, Command: command}, true
}

func parseMCPProxyProcessSnapshot(line, binary string) (mcpProxyProcess, bool) {
	line = strings.TrimSpace(line)
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return mcpProxyProcess{}, false
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil || pid <= 0 {
		return mcpProxyProcess{}, false
	}
	parentPID, err := strconv.Atoi(fields[1])
	if err != nil || parentPID < 0 {
		return mcpProxyProcess{}, false
	}
	command := strings.Join(fields[2:], " ")
	if command != binary+" mcp" {
		return mcpProxyProcess{}, false
	}
	return mcpProxyProcess{PID: pid, ParentPID: parentPID, Command: command}, true
}

func terminateMCPProxyProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Signal(syscall.SIGTERM)
}
