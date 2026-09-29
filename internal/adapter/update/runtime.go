package update

import (
	"bytes"
	"fmt"
	"io"
	daemoncontract "issueops/internal/contract/daemon"
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
	InspectProcess      func(int) (daemoncontract.ProcessIdentity, error)
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
func (r Runtime) StopDaemon() error {
	return r.RunInstalledDaemon(filepath.Join(r.Root, "bin", "issueops"), "daemon", "stop", "--json")
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
func (r Runtime) RunInstalledDaemon(binary string, args ...string) error {
	cmd := exec.Command(binary, args...)
	cmd.Dir = r.Root
	cmd.Env = r.Environment
	cmd.Stdout = io.Discard
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		command := strings.Join(args, " ")
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			return fmt.Errorf("run installed daemon command %q: %w", command, err)
		}
		return fmt.Errorf("run installed daemon command %q: %w: %s", command, err, detail)
	}
	return nil
}

func (r Runtime) Daemons() ([]contract.DaemonProcess, error) {
	out, err := r.ProcessTable.Read("-axo", "pid=,command=")
	if err != nil {
		// ps may be unavailable in sandboxed environments; treat as no matching processes.
		return nil, nil
	}
	binary := filepath.Join(r.Root, "bin", "issueops")
	var processes []contract.DaemonProcess
	for _, line := range strings.Split(string(out), "\n") {
		process, ok := ParseDaemonProcess(line, binary)
		if ok {
			processes = append(processes, process)
		}
	}
	return processes, nil
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

func ParseDaemonProcess(line, binary string) (contract.DaemonProcess, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return contract.DaemonProcess{}, false
	}
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return contract.DaemonProcess{}, false
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return contract.DaemonProcess{}, false
	}
	command := strings.Join(fields[1:], " ")
	if command != binary+" daemon --internal" {
		return contract.DaemonProcess{}, false
	}
	return contract.DaemonProcess{PID: pid, Command: command}, true
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
