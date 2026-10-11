package update

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"issueops/internal/contract/processidentity"
	contract "issueops/internal/contract/update"
)

func writeScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
}

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
	t.Parallel()
	dir := t.TempDir()
	script := filepath.Join(dir, "ps")
	writeScript(t, script, "echo \"$@|$FOO\"\n")

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

func TestRuntimeInstall(t *testing.T) {
	t.Parallel()
	t.Run("runs script with args, env, stdin in root", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeScript(t, filepath.Join(root, "scripts", "install-native.sh"),
			"read line; echo \"args=$@ env=$FOO in=$line pwd=$(pwd -P)\"; echo err >&2\n")
		var out, diag bytes.Buffer
		rt := Runtime{Environment: []string{"FOO=bar"}, Input: strings.NewReader("hello\n"), Output: &out, Diagnostics: &diag}
		if err := rt.Install(root, []string{"--a", "b"}); err != nil {
			t.Fatal(err)
		}
		realRoot, _ := filepath.EvalSymlinks(root)
		if want := "args=--a b env=bar in=hello pwd=" + realRoot + "\n"; out.String() != want {
			t.Fatalf("stdout %q want %q", out.String(), want)
		}
		if diag.String() != "err\n" {
			t.Fatalf("stderr %q", diag.String())
		}
	})
	t.Run("missing script", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		err := Runtime{}.Install(root, nil)
		if err == nil || !strings.Contains(err.Error(), "install script not found at "+filepath.Join(root, "scripts", "install-native.sh")) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("script failure propagates", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeScript(t, filepath.Join(root, "scripts", "install-native.sh"), "exit 3\n")
		if err := (Runtime{}).Install(root, nil); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestRuntimeList(t *testing.T) {
	t.Parallel()
	newRoot := func(t *testing.T) (root, bin string) {
		t.Helper()
		root = t.TempDir()
		real, err := filepath.EvalSymlinks(root)
		if err != nil {
			t.Fatal(err)
		}
		bin = filepath.Join(real, "bin", "issueops")
		writeScript(t, bin, "exit 0\n")
		return real, bin
	}
	table := func(t *testing.T, body string) ProcessTable {
		t.Helper()
		p := filepath.Join(t.TempDir(), "ps")
		writeScript(t, p, body)
		return ProcessTable{Path: p}
	}

	t.Run("filters, verifies identity, tolerates inspect errors", func(t *testing.T) {
		t.Parallel()
		root, bin := newRoot(t)
		body := "cat <<'EOF'\n" +
			"  10 1 " + bin + " mcp\n" +
			"  11 1 " + bin + " mcp\n" +
			"  12 1 " + bin + " mcp\n" +
			"  13 1 /other mcp\n" +
			"garbage\n" +
			"EOF\n"
		rt := Runtime{Root: root, ProcessTable: table(t, body),
			InspectProcess: func(pid int) (processidentity.Identity, error) {
				switch pid {
				case 10:
					return processidentity.Identity{StartTime: "t10", Executable: bin}, nil
				case 11:
					return processidentity.Identity{StartTime: "t11", Executable: "/elsewhere"}, nil
				}
				return processidentity.Identity{}, errors.New("gone")
			}}
		got, err := rt.List()
		if err != nil {
			t.Fatal(err)
		}
		cmd := bin + " mcp"
		want := []contract.MCPProxyProcess{
			{PID: 10, ParentPID: 1, Command: cmd, StartTime: "t10", Executable: bin, IdentityVerified: true},
			{PID: 11, ParentPID: 1, Command: cmd, StartTime: "t11", Executable: "/elsewhere"},
			{PID: 12, ParentPID: 1, Command: cmd},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v\nwant %+v", got, want)
		}
	})
	t.Run("no matches yields nil", func(t *testing.T) {
		t.Parallel()
		root, _ := newRoot(t)
		got, err := Runtime{Root: root, ProcessTable: table(t, "echo nothing\n")}.List()
		if err != nil || got != nil {
			t.Fatalf("got %v, %v", got, err)
		}
	})
	t.Run("inventory failure wrapped", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("denied")
		_, err := Runtime{ProcessTable: ProcessTable{PathError: sentinel}}.List()
		if !errors.Is(err, sentinel) || !strings.HasPrefix(err.Error(), "MCP proxy inventory failed: ") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("missing binary", func(t *testing.T) {
		t.Parallel()
		_, err := Runtime{Root: t.TempDir(), ProcessTable: table(t, "echo\n")}.List()
		if err == nil || !strings.HasPrefix(err.Error(), "resolve MCP proxy binary: ") {
			t.Fatalf("got %v", err)
		}
	})
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
