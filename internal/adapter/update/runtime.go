package update

import (
	"fmt"
	"io"
	"issueops/internal/contract/processidentity"
	contract "issueops/internal/contract/update"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

type ProcessTable struct {
	Path        string
	PathError   error
	Environment []string
}

func (p ProcessTable) Read(args ...string) ([]byte, error) {
	if p.PathError != nil {
		return nil, p.PathError
	}
	command := exec.Command(p.Path, args...)
	command.Env = p.Environment
	return command.Output()
}

type Runtime struct {
	Root                string
	Environment         []string
	Input               io.Reader
	Output, Diagnostics io.Writer
	ProcessTable        ProcessTable
	InspectProcess      func(int) (processidentity.Identity, error)
}

func (r Runtime) Install(root string, args []string) error {
	script := filepath.Join(root, "scripts", "install-native.sh")
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("install script not found at %s: %w", script, err)
	}
	command := exec.Command(script, args...)
	command.Dir = root
	command.Env = r.Environment
	command.Stdin = r.Input
	command.Stdout = r.Output
	command.Stderr = r.Diagnostics
	return command.Run()
}
func (Runtime) CurrentPID() int                 { return os.Getpid() }
func (Runtime) SupportsOrphanTermination() bool { return runtime.GOOS == "darwin" }
func (Runtime) Terminate(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Signal(syscall.SIGTERM)
}

func (r Runtime) List() ([]contract.MCPProxyProcess, error) {
	out, err := r.ProcessTable.Read("-axo", "pid=,ppid=,command=")
	if err != nil {
		return nil, fmt.Errorf("MCP proxy inventory failed: %w", err)
	}
	binary := filepath.Join(r.Root, "bin", "issueops")
	canonicalBinary, err := filepath.EvalSymlinks(binary)
	if err != nil {
		return nil, fmt.Errorf("resolve MCP proxy binary: %w", err)
	}
	var processes []contract.MCPProxyProcess
	for _, line := range strings.Split(string(out), "\n") {
		process, ok := ParseMCPProxyProcessSnapshot(line, canonicalBinary)
		if !ok {
			continue
		}
		identity, identityErr := r.InspectProcess(process.PID)
		if identityErr == nil {
			process.StartTime = identity.StartTime
			process.Executable = identity.Executable
			process.IdentityVerified = identity.Executable == canonicalBinary
		}
		processes = append(processes, process)
	}
	return processes, nil
}

func ParseMCPProxyProcessSnapshot(line, binary string) (contract.MCPProxyProcess, bool) {
	line = strings.TrimSpace(line)
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return contract.MCPProxyProcess{}, false
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil || pid <= 0 {
		return contract.MCPProxyProcess{}, false
	}
	parentPID, err := strconv.Atoi(fields[1])
	if err != nil || parentPID < 0 {
		return contract.MCPProxyProcess{}, false
	}
	command := strings.Join(fields[2:], " ")
	if command != binary+" mcp" {
		return contract.MCPProxyProcess{}, false
	}
	return contract.MCPProxyProcess{PID: pid, ParentPID: parentPID, Command: command}, true
}
