// Package update is the adapter behind the update command: it supplies the
// process inventory, process termination, and native install steps that the
// update application service orchestrates. It is the only layer that shells out
// to ps or the install script; unix and windows builds implement Runtime.Install
// separately.
package update

import (
	"fmt"
	"io"
	"issueops/internal/adapter/outbound/osfile"
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

// ProcessTable runs the external process-listing command (ps on unix) for Runtime.
// A non-nil PathError means the command could not be resolved and is returned from every Read.
type ProcessTable struct {
	Path        string
	PathError   error
	Environment []string
}

// Read runs the process-listing command with args in Environment and returns its stdout.
func (p ProcessTable) Read(args ...string) ([]byte, error) {
	if p.PathError != nil {
		return nil, p.PathError
	}
	command := exec.Command(p.Path, args...)
	command.Env = p.Environment
	return command.Output()
}

// Runtime is the host-side implementation of the update service's runtime port.
// Root is the issueops installation root; Input, Output and Diagnostics are wired
// to the install step's standard streams; InspectProcess verifies the identity of a listed MCP proxy process.
type Runtime struct {
	Root                string
	Environment         []string
	Input               io.Reader
	Output, Diagnostics io.Writer
	ProcessTable        ProcessTable
	InspectProcess      func(int) (processidentity.Identity, error)
}

// CurrentPID returns the PID of the running process.
func (Runtime) CurrentPID() int { return os.Getpid() }

// SupportsOrphanTermination reports whether orphaned MCP proxies can be terminated on this OS (darwin only).
func (Runtime) SupportsOrphanTermination() bool { return runtime.GOOS == "darwin" }

// Terminate asks the process with the given PID to exit by sending SIGTERM.
func (Runtime) Terminate(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Signal(syscall.SIGTERM)
}

// List returns the MCP proxy processes ("<root>/bin/issueops mcp") currently running.
// It fails when the process table cannot be read or the installed binary cannot be resolved.
// IdentityVerified is set only when the inspected executable matches the canonical installed binary.
func (r Runtime) List() ([]contract.MCPProxyProcess, error) {
	out, err := r.processRows()
	if err != nil {
		return nil, fmt.Errorf("MCP proxy inventory failed: %w", err)
	}
	binary := filepath.Join(r.Root, "bin", osfile.ExecutableName("issueops"))
	canonicalBinary, err := filepath.EvalSymlinks(binary)
	if err != nil {
		return nil, fmt.Errorf("resolve MCP proxy binary: %w", err)
	}
	var processes []contract.MCPProxyProcess
	for line := range strings.SplitSeq(string(out), "\n") {
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

// ParseMCPProxyProcessSnapshot parses one "pid ppid command..." process-table row.
// It returns false unless the command is exactly the canonical binary followed by "mcp" and both ids are valid.
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
