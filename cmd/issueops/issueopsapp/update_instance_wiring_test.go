package issueopsapp

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"issueops/cmd/issueops/updatecli"
	contract "issueops/internal/contract/update"
)

func TestUpdateRootCommandsKeepCapturedContext(t *testing.T) {
	if os.Getenv("ISSUEOPS_UPDATE_TEST_CHILD") != "1" {
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(exe, "-test.run=^TestUpdateRootCommandsKeepCapturedContext$", "-test.count=1")
		cmd.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "TMPDIR=/tmp", "HOME=" + t.TempDir(), "ISSUEOPS_UPDATE_TEST_CHILD=1"}
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("isolated update context test: %v\n%s", err, output)
		}
		return
	}
	var commands [2]updatecli.Command
	var cleanup [2]updatecli.CleanupCommand
	var roots [2]string
	for i := range commands {
		var err error
		roots[i], err = filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		script := filepath.Join(roots[i], "scripts", "install-native.sh")
		binary := filepath.Join(roots[i], "bin", "issueops")
		ps := filepath.Join(roots[i], "bin", "ps")
		receipt := "#!/bin/sh\nprintf '%s\\n' \"$PWD\" \"$ISSUEOPS_ROOT\" \"$UPDATE_TEST_MARKER\" \"$@\" > \"$0.receipt\"\n"
		writeUpdateFixture(t, script, receipt)
		writeUpdateFixture(t, binary, receipt)
		writeUpdateFixture(t, ps, "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$0.receipt\"\ncase \"$*\" in\n '-axo pid=,command=') printf '2147483000 %s mcp\\n' \"$ISSUEOPS_ROOT/bin/issueops\";;\n '-axo pid=,ppid=,command=') printf '2147483000 1 %s mcp\\n' \"$ISSUEOPS_ROOT/bin/issueops\";;\n '-p 2147483000 -o lstart=') printf 'Tue Sep 29 12:00:00 2026\\n';;\n '-p 2147483000 -o comm=') printf '%s/bin/issueops\\n' \"$ISSUEOPS_ROOT\";;\n *) exit 23;;\nesac\n")
		t.Setenv("ISSUEOPS_ROOT", roots[i])
		t.Setenv("UPDATE_TEST_MARKER", strconv.Itoa(i))
		t.Setenv("PATH", filepath.Dir(ps)+":/usr/bin:/bin")
		commands[i] = newUpdateCommand()
		cleanup[i] = newMCPCleanupCommand()
	}
	t.Setenv("ISSUEOPS_ROOT", t.TempDir())
	t.Setenv("UPDATE_TEST_MARKER", "changed")
	t.Setenv("PATH", "/usr/bin:/bin")
	t.Chdir(t.TempDir())
	for i, command := range commands {
		args := []string{"--project-local", "--interactive", "--json", "--path-mode=skip", "--skip-build"}
		if err := command.Run("update", args); err != nil {
			t.Fatal(err)
		}
		script := filepath.Join(roots[i], "scripts", "install-native.sh")
		binary := filepath.Join(roots[i], "bin", "issueops")
		ps := filepath.Join(roots[i], "bin", "ps")
		assertUpdateReceipt(t, script, roots[i], i, []string{"--project-local", "--path-mode=skip", "--interactive", "--json", "--skip-build"})
		assertUpdateReceipt(t, binary, roots[i], i, []string{"daemon", "stop", "--json"})
		before, err := os.ReadFile(ps + ".receipt")
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != "-axo pid=,command=\n" {
			t.Fatalf("update inspected MCP processes: %q", before)
		}
		if err := command.Run("bootstrap", []string{"--dry-run", "--json"}); err != nil {
			t.Fatal(err)
		}
		assertUpdateReceipt(t, script, roots[i], i, []string{"--dry-run", "--json"})
		after, err := os.ReadFile(ps + ".receipt")
		if err != nil || string(after) != string(before) {
			t.Fatalf("dry-run observed processes: %s %v", after, err)
		}
		raw := captureStdoutForContract(t, func() error { return cleanup[i].Run([]string{"--json"}) })
		var result contract.MCPCleanupResult
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			t.Fatal(err)
		}
		wantAction := "skip-unverified"
		if runtime.GOOS == "darwin" {
			wantAction = "would-terminate"
		}
		if !result.OK || !result.DryRun || result.Matched != 1 || result.Terminated != 0 || len(result.Processes) != 1 || result.Processes[0].Command != binary+" mcp" || result.Processes[0].Action != wantAction {
			t.Fatalf("cleanup %d leaked its root or process-table configuration: %+v", i, result)
		}
	}
}

func writeUpdateFixture(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
}
func assertUpdateReceipt(t *testing.T, path, root string, index int, args []string) {
	t.Helper()
	raw, err := os.ReadFile(path + ".receipt")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("receipt %s: %q", path, raw)
	}
	observed, err := filepath.EvalSymlinks(lines[0])
	if err != nil {
		t.Fatal(err)
	}
	expected, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if observed != expected || lines[1] != root || lines[2] != strconv.Itoa(index) || !reflect.DeepEqual(lines[3:], args) {
		t.Fatalf("wrong captured execution for %s: %q", path, lines)
	}
}
