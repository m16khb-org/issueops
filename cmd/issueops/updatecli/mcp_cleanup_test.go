package updatecli

import (
	"encoding/json"
	"errors"
	updatecontract "issueops/internal/contract/update"
	"issueops/internal/testsupport"
	"strings"
	"testing"
)

type fakeProxyEffects struct {
	procs      []updatecontract.MCPProxyProcess
	listErr    error
	terminated []int
}

func (f *fakeProxyEffects) List() ([]updatecontract.MCPProxyProcess, error) {
	return f.procs, f.listErr
}
func (f *fakeProxyEffects) Terminate(pid int) error {
	f.terminated = append(f.terminated, pid)
	return nil
}
func (f *fakeProxyEffects) CurrentPID() int                 { return 1 }
func (f *fakeProxyEffects) SupportsOrphanTermination() bool { return true }

func orphanProxy() updatecontract.MCPProxyProcess {
	return updatecontract.MCPProxyProcess{PID: 4242, ParentPID: 1, Command: "/bin/issueops mcp", StartTime: "t0", Executable: "/bin/issueops", IdentityVerified: true}
}

func TestCleanupCommandRejectsBadArguments(t *testing.T) {
	cmd := CleanupCommand{Effects: &fakeProxyEffects{}}
	for name, args := range map[string][]string{
		"unknown flag":    {"--bogus"},
		"positional":      {"extra"},
		"dry-run & apply": {"--dry-run", "--apply"},
	} {
		if err := cmd.Run(args); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestCleanupCommandDefaultsToDryRunJSON(t *testing.T) {
	effects := &fakeProxyEffects{procs: []updatecontract.MCPProxyProcess{orphanProxy()}}
	var err error
	out := testsupport.CaptureStdout(t, func() error { err = CleanupCommand{Effects: effects}.Run([]string{"--json"}); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(effects.terminated) != 0 {
		t.Fatalf("dry run terminated %v", effects.terminated)
	}
	var result updatecontract.MCPCleanupResult
	if jsonErr := json.Unmarshal([]byte(out), &result); jsonErr != nil {
		t.Fatalf("not JSON: %v\n%s", jsonErr, out)
	}
	if !result.DryRun || result.Matched != 1 || result.Processes[0].Action != "would-terminate" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCleanupCommandApplyTerminatesAndPrintsText(t *testing.T) {
	effects := &fakeProxyEffects{procs: []updatecontract.MCPProxyProcess{orphanProxy()}}
	var err error
	out := testsupport.CaptureStdout(t, func() error { err = CleanupCommand{Effects: effects}.Run([]string{"--apply"}); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(effects.terminated) != 1 || effects.terminated[0] != 4242 {
		t.Fatalf("expected pid 4242 terminated, got %v", effects.terminated)
	}
	want := "MCP proxy cleanup complete: matched=1 terminated=1 dry_run=false"
	if !strings.Contains(out, want) {
		t.Fatalf("output %q missing %q", out, want)
	}
}

func TestCleanupCommandReturnsListErrorAfterPrintingJSON(t *testing.T) {
	boom := errors.New("ps failed")
	var err error
	out := testsupport.CaptureStdout(t, func() error {
		err = CleanupCommand{Effects: &fakeProxyEffects{listErr: boom}}.Run([]string{"--json"})
		return nil
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected list error, got %v", err)
	}
	if !strings.Contains(out, `"ok": false`) && !strings.Contains(out, `"ok":false`) {
		t.Fatalf("expected ok=false JSON, got %s", out)
	}
}
