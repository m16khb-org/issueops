package issueopsapp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	guardcontract "issueops/internal/contract/guard"
	tracecontract "issueops/internal/contract/trace"
	"issueops/internal/testsupport"
)

func TestBasicCommandsKeepCapturedRootsAndStores(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ISSUEOPS_ROOT", root)
	t.Setenv("CLAUDE_PROJECT_DIR", root)
	state := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", state)
	if _, err := newStateService(state).Write("basic-trace", `{"failed_steps":1,"failed_step":"policy"}`); err != nil {
		t.Fatal(err)
	}
	stateRoot := issueOpsStateRoot()
	record := seedReleasedDirectHandoffRecord(t, stateRoot)
	first := newBasicCommand()
	other := t.TempDir()
	t.Setenv("ISSUEOPS_ROOT", other)
	t.Setenv("CLAUDE_PROJECT_DIR", other)
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	second := newBasicCommand()
	if first.IssueOpsRoot != root || first.DefaultTarget != root || second.IssueOpsRoot != other || second.DefaultTarget != other {
		t.Fatal("command roots followed another command")
	}
	output, err := testsupport.CaptureStdoutAndError(t, func() error { return first.RunTrace([]string{"analyze", "--json", "basic-trace"}) })
	if err != nil {
		t.Fatal(err)
	}
	var result tracecontract.TraceAnalyzeResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatal(err)
	}
	if result.InputSource != "state" || len(result.Findings) != 1 || result.Findings[0].RecurringPattern != "policy" {
		t.Fatalf("trace=%+v", result)
	}
	if _, err := first.Handoff.ObserveManual(manualCmuxHandoffObservation(record.ID, record.Execution.Lease.Generation)); err != nil {
		t.Fatal(err)
	}
	observations, err := newHandoffDeliveryAudit(stateRoot).Read()
	if err != nil || len(observations) != 1 {
		t.Fatalf("handoff namespace records=%d err=%v", len(observations), err)
	}
	ambient, err := newHandoffDeliveryAudit(issueOpsStateRoot()).Read()
	if err != nil || len(ambient) != 0 {
		t.Fatalf("ambient state touched: %d %v", len(ambient), err)
	}

	for _, test := range []struct {
		command func([]string) error
		root    string
	}{{first.RunInspect, root}, {second.RunInspect, other}} {
		output, err := testsupport.CaptureStdoutAndError(t, func() error { return test.command(nil) })
		if err != nil || !strings.Contains(output, "issueops root: "+test.root) || !strings.Contains(output, "target repo: "+test.root) {
			t.Fatalf("inspect=%s err=%v", output, err)
		}
	}
}

func TestBasicGuardKeepsCapturedWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.WriteFile(filepath.Join(root, "slow_test.go"), []byte("package main\nfunc TestSlow(t *testing.T) { time.Sleep(1) }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	command := newBasicCommand()
	t.Chdir(t.TempDir())
	output, err := testsupport.CaptureStdoutAndError(t, func() error { return command.RunGuard([]string{"check", "--json", "--", "slow_test.go"}) })
	if !guardcontract.IsGuardBlocked(err) {
		t.Fatalf("expected blocked error, got %v", err)
	}
	var result guardcontract.GuardCheckResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatal(err)
	}
	got, _ := filepath.EvalSymlinks(result.RepoRoot)
	want, _ := filepath.EvalSymlinks(root)
	if got != want || result.Summary.Block != 1 {
		t.Fatalf("guard=%+v", result)
	}
}
