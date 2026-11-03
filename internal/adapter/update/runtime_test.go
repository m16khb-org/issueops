package update

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	contract "issueops/internal/contract/update"
	"issueops/internal/testsupport"
)

func TestParseMCPProxyProcessSnapshot(t *testing.T) {
	t.Parallel()
	const bin = "/opt/issueops/bin/issueops"
	tests := []struct {
		name string
		line string
		want contract.MCPProxyProcess
		ok   bool
	}{
		{"valid", "  123 45 " + bin + " mcp", contract.MCPProxyProcess{PID: 123, ParentPID: 45, Command: bin + " mcp"}, true},
		{"parent zero allowed", "9 0 " + bin + " mcp", contract.MCPProxyProcess{PID: 9, Command: bin + " mcp"}, true},
		{"extra whitespace collapsed", "9\t1   " + bin + "   mcp  ", contract.MCPProxyProcess{PID: 9, ParentPID: 1, Command: bin + " mcp"}, true},
		{"too few fields", "1 2 " + bin, contract.MCPProxyProcess{}, false},
		{"empty", "", contract.MCPProxyProcess{}, false},
		{"non numeric pid", "x 2 " + bin + " mcp", contract.MCPProxyProcess{}, false},
		{"zero pid", "0 2 " + bin + " mcp", contract.MCPProxyProcess{}, false},
		{"negative ppid", "1 -2 " + bin + " mcp", contract.MCPProxyProcess{}, false},
		{"non numeric ppid", "1 y " + bin + " mcp", contract.MCPProxyProcess{}, false},
		{"other binary", "1 2 /usr/bin/other mcp", contract.MCPProxyProcess{}, false},
		{"extra args", "1 2 " + bin + " mcp --x", contract.MCPProxyProcess{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ParseMCPProxyProcessSnapshot(tt.line, bin)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("got %+v,%v want %+v,%v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestProcessTableRead(t *testing.T) {
	dir := t.TempDir()
	script := testsupport.WriteFakeCommand(t, dir, "ps", "#!/bin/sh\necho \"$@|$FOO\"\n")

	out, err := ProcessTable{Path: script, Environment: []string{"FOO=bar"}}.Read("-a", "b")
	if err != nil || string(out) != "-a b|bar\n" {
		t.Fatalf("got %q, %v", out, err)
	}
	sentinel := errors.New("no ps")
	if _, err := (ProcessTable{Path: script, PathError: sentinel}).Read(); !errors.Is(err, sentinel) {
		t.Fatalf("want PathError, got %v", err)
	}
	if _, err := (ProcessTable{Path: filepath.Join(dir, "missing")}).Read(); err == nil {
		t.Fatal("expected error for missing executable")
	}
}

func TestRuntimeProcessBasics(t *testing.T) {
	t.Parallel()
	rt := Runtime{}
	if rt.CurrentPID() != os.Getpid() {
		t.Fatal("CurrentPID must be os.Getpid")
	}
	if err := rt.Terminate(-1 << 30); err == nil {
		t.Fatal("terminating a nonexistent pid must fail")
	}
}
