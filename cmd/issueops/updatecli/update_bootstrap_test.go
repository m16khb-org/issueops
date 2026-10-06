package updatecli

import (
	contract "issueops/internal/contract/update"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func stubInstallScriptCommandRunner(t *testing.T, fn func(string, ...string) error) func() {
	t.Helper()
	previous := installScriptCommandRunner
	installScriptCommandRunner = fn
	return func() { installScriptCommandRunner = previous }
}

func stubMCPProxyProcessLister(t *testing.T, fn func() ([]contract.MCPProxyProcess, error)) func() {
	t.Helper()
	previous := mcpProxyProcessLister
	mcpProxyProcessLister = fn
	return func() { mcpProxyProcessLister = previous }
}

func stubMCPProxyTerminator(t *testing.T, fn func(int) error) func() {
	t.Helper()
	previous := mcpProxyTerminator
	mcpProxyTerminator = fn
	return func() { mcpProxyTerminator = previous }
}

func TestRunInstallScriptCommandRunsOnlyTheInstallScript(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ISSUEOPS_ROOT", root)
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "install-native.sh"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	var commands []string
	restore := stubInstallScriptCommandRunner(t, func(name string, args ...string) error {
		commands = append(commands, append([]string{name}, args...)...)
		return nil
	})
	defer restore()

	if err := runInstallScriptCommand("update", nil); err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(root, "scripts", "install-native.sh")}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("unexpected command sequence:\n got: %#v\nwant: %#v", commands, want)
	}
}

func TestRunUpdateAndBootstrapForwardToInstallScript(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ISSUEOPS_ROOT", root)
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "install-native.sh"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	var commands [][]string
	restore := stubInstallScriptCommandRunner(t, func(name string, args ...string) error {
		commands = append(commands, append([]string{name}, args...))
		return nil
	})
	defer restore()

	if err := runUpdate([]string{"--dry-run", "--json"}); err != nil {
		t.Fatal(err)
	}
	if err := runBootstrap([]string{"--dry-run", "--path-mode=skip", "--skip-build"}); err != nil {
		t.Fatal(err)
	}

	want := [][]string{
		{filepath.Join(root, "scripts", "install-native.sh"), "--dry-run", "--json"},
		{filepath.Join(root, "scripts", "install-native.sh"), "--dry-run", "--path-mode=skip", "--skip-build"},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("unexpected wrapper command sequence:\n got: %#v\nwant: %#v", commands, want)
	}
}

func TestRunUpdateUsesResolvedIssueOpsRootOutsideCheckout(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "scripts", "install-native.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldCWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(outside); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldCWD) })
	Configure(Deps{IssueOpsRoot: func() string { return root }})
	t.Cleanup(Reset)

	var got string
	restore := stubInstallScriptCommandRunner(t, func(name string, args ...string) error {
		got = name
		return nil
	})
	defer restore()

	if err := runUpdate([]string{"--dry-run", "--path-mode=skip"}); err != nil {
		t.Fatal(err)
	}
	if got != script {
		t.Fatalf("update script = %q, want %q", got, script)
	}
}

func TestCleanupMCPProxiesDryRunAndApply(t *testing.T) {
	previousSupport := mcpProxyOrphanTerminationSupported
	mcpProxyOrphanTerminationSupported = func() bool { return true }
	t.Cleanup(func() {
		mcpProxyOrphanTerminationSupported = previousSupport
	})
	binary := "/repo/bin/issueops"
	processes := []contract.MCPProxyProcess{
		{
			PID: os.Getpid(), ParentPID: 1, Command: binary + " mcp",
			StartTime: "current-start", Executable: binary, IdentityVerified: true,
		},
		{
			PID: 22, ParentPID: 900, Command: binary + " mcp",
			StartTime: "live-start", Executable: binary, IdentityVerified: true,
		},
		{
			PID: 33, ParentPID: 1, Command: binary + " mcp",
			StartTime: "orphan-start", Executable: binary, IdentityVerified: true,
		},
		{
			PID: 44, ParentPID: 1, Command: binary + " mcp",
			StartTime: "", Executable: "", IdentityVerified: false,
		},
		{
			PID: 55, ParentPID: 1, Command: "npm exec @upstash/context7-mcp",
			StartTime: "external-start", Executable: "/usr/local/bin/node", IdentityVerified: true,
		},
	}
	restoreList := stubMCPProxyProcessLister(t, func() ([]contract.MCPProxyProcess, error) {
		return append([]contract.MCPProxyProcess(nil), processes...), nil
	})
	defer restoreList()

	var terminated []int
	restoreTerm := stubMCPProxyTerminator(t, func(pid int) error {
		terminated = append(terminated, pid)
		return nil
	})
	defer restoreTerm()

	dryRun, err := CleanupMCPProxies(true)
	if err != nil {
		t.Fatal(err)
	}
	if !dryRun.OK || !dryRun.DryRun || dryRun.Matched != 5 || dryRun.Terminated != 0 || len(terminated) != 0 {
		t.Fatalf("dry-run cleanup = %#v terminated=%#v", dryRun, terminated)
	}
	wantDryRunActions := []string{"skip-current", "skip-live-parent", "would-terminate", "skip-unverified", "skip-not-exact"}
	if got := mcpCleanupActions(dryRun.Processes); !reflect.DeepEqual(got, wantDryRunActions) {
		t.Fatalf("dry-run cleanup actions = %#v", dryRun.Processes)
	}

	applied, err := CleanupMCPProxies(false)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.OK || applied.DryRun || applied.Matched != 5 || applied.Terminated != 1 {
		t.Fatalf("apply cleanup = %#v", applied)
	}
	if !reflect.DeepEqual(terminated, []int{33}) {
		t.Fatalf("terminated = %#v", terminated)
	}
	wantApplyActions := []string{"skip-current", "skip-live-parent", "terminated", "skip-unverified", "skip-not-exact"}
	if got := mcpCleanupActions(applied.Processes); !reflect.DeepEqual(got, wantApplyActions) {
		t.Fatalf("apply cleanup actions = %#v", applied.Processes)
	}
}

func TestCleanupMCPProxiesSkipsIdentityChangedBeforeSignal(t *testing.T) {
	previousSupport := mcpProxyOrphanTerminationSupported
	mcpProxyOrphanTerminationSupported = func() bool { return true }
	t.Cleanup(func() {
		mcpProxyOrphanTerminationSupported = previousSupport
	})
	binary := "/repo/bin/issueops"
	first := contract.MCPProxyProcess{
		PID: 33, ParentPID: 1, Command: binary + " mcp",
		StartTime: "orphan-start", Executable: binary, IdentityVerified: true,
	}
	second := first
	second.StartTime = "reused-pid-start"
	listCalls := 0
	restoreList := stubMCPProxyProcessLister(t, func() ([]contract.MCPProxyProcess, error) {
		listCalls++
		if listCalls == 1 {
			return []contract.MCPProxyProcess{first}, nil
		}
		return []contract.MCPProxyProcess{second}, nil
	})
	defer restoreList()
	restoreTerm := stubMCPProxyTerminator(t, func(pid int) error {
		t.Fatalf("identity-changed pid %d must not be signaled", pid)
		return nil
	})
	defer restoreTerm()

	result, err := CleanupMCPProxies(false)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Terminated != 0 || len(result.Processes) != 1 ||
		result.Processes[0].Action != "skip-identity-changed" {
		t.Fatalf("identity-changed cleanup = %#v", result)
	}
}

func TestCleanupMCPProxiesSkipsOrphansOnUnsupportedPlatforms(t *testing.T) {
	previousSupport := mcpProxyOrphanTerminationSupported
	mcpProxyOrphanTerminationSupported = func() bool { return false }
	t.Cleanup(func() {
		mcpProxyOrphanTerminationSupported = previousSupport
	})
	binary := "/repo/bin/issueops"
	restoreList := stubMCPProxyProcessLister(t, func() ([]contract.MCPProxyProcess, error) {
		return []contract.MCPProxyProcess{{
			PID:              33,
			ParentPID:        1,
			Command:          binary + " mcp",
			StartTime:        "orphan-start",
			Executable:       binary,
			IdentityVerified: true,
		}}, nil
	})
	defer restoreList()
	restoreTerm := stubMCPProxyTerminator(t, func(pid int) error {
		t.Fatalf("unsupported platform pid %d must not be signaled", pid)
		return nil
	})
	defer restoreTerm()

	result, err := CleanupMCPProxies(false)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Terminated != 0 || len(result.Processes) != 1 ||
		result.Processes[0].Action != "skip-unsupported-platform" {
		t.Fatalf("unsupported-platform cleanup = %#v", result)
	}
}

func TestCleanupMCPProxiesReturnsEmptyProcessListWhenNoMatches(t *testing.T) {
	restoreList := stubMCPProxyProcessLister(t, func() ([]contract.MCPProxyProcess, error) {
		return nil, nil
	})
	defer restoreList()

	result, err := CleanupMCPProxies(true)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Matched != 0 || result.Processes == nil || len(result.Processes) != 0 {
		t.Fatalf("cleanup no matches = %#v", result)
	}
}

func mcpCleanupActions(processes []contract.MCPCleanupProcess) []string {
	actions := make([]string, 0, len(processes))
	for _, process := range processes {
		actions = append(actions, process.Action)
	}
	return actions
}
