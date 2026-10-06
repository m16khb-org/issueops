package updatecli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMCPProxyProcessOnlyMatchesCurrentHarnessMCP(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "bin", "issueops")
	match, ok := parseMCPProxyProcessSnapshot("  123 1 "+binary+" mcp", binary)
	if !ok || match.PID != 123 || match.Command != binary+" mcp" {
		t.Fatalf("expected MCP proxy match, got match=%+v ok=%v", match, ok)
	}
	for _, line := range []string{
		"124 1 " + binary + " worker run",
		"125 1 " + binary + " update",
		"126 1 /other/bin/issueops mcp",
		"not-a-pid 1 " + binary + " mcp",
	} {
		if got, ok := parseMCPProxyProcessSnapshot(line, binary); ok {
			t.Fatalf("unexpected match for %q: %+v", line, got)
		}
	}
}

func TestParseMCPProxyProcessSnapshotRequiresExactHarnessCommand(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "bin", "issueops")
	process, ok := parseMCPProxyProcessSnapshot("123 1 "+binary+" mcp", binary)
	if !ok || process.PID != 123 || process.ParentPID != 1 || process.Command != binary+" mcp" {
		t.Fatalf("exact snapshot parse = %#v ok=%v", process, ok)
	}
	for _, line := range []string{
		"124 1 npm exec @upstash/context7-mcp",
		"125 1 node /tmp/node_modules/.bin/dbhub",
		"126 900 " + binary + " worker run",
		"127 1 /other/bin/issueops mcp",
	} {
		if process, ok := parseMCPProxyProcessSnapshot(line, binary); ok {
			t.Fatalf("external or non-proxy process matched %q: %#v", line, process)
		}
	}
}

func TestRefreshRunningMCPProxiesAfterInstallPreservesAllActiveProcesses(t *testing.T) {
	restoreList := stubMCPProxyProcessLister(t, func() ([]mcpProxyProcess, error) {
		t.Fatal("post-install refresh must not enumerate host-owned MCP processes")
		return nil, nil
	})
	defer restoreList()
	var terminated []int
	restoreTerminate := stubMCPProxyTerminator(t, func(pid int) error {
		terminated = append(terminated, pid)
		return nil
	})
	defer restoreTerminate()

	root := t.TempDir()
	t.Setenv("ISSUEOPS_ROOT", root)
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "install-native.sh"), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	restoreInstall := stubInstallScriptCommandRunner(t, func(string, ...string) error { return nil })
	defer restoreInstall()
	err := runInstallScriptCommand("update", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(terminated) != 0 {
		t.Fatalf("update terminated active MCP processes: %v", terminated)
	}
}
