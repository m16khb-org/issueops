package operationalhealth

import (
	"context"
	"errors"
	operationalhealthcontract "issueops/internal/contract/operationalhealth"
	"testing"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
	corehealth "issueops/internal/domain/operationalhealth"
)

func TestCycleFromRecordProjectsExecution(t *testing.T) {
	record := issueopscontract.IssueOpsRecord{
		ID: "io-v1", Repo: "/repo", Branch: "69-v1", Phase: issueopscontract.IssueOpsPhaseImplement,
		Execution: &issueopscontract.Execution{
			Workspace: issueopscontract.Workspace{Root: "/repo.worktrees/69-v1"},
			Lease: issueopscontract.WriteLease{
				Generation: 3, Status: issueopscontract.LeaseStatusActive,
				Holder: &issueopscontract.NativeActor{Host: "codex", SessionID: "session-v1", AgentID: "agent-v1", SessionProcess: &issueopscontract.NativeProcessReceipt{
					PID: 123, StartedAt: "2026-07-22T00:00:00Z", Executable: "/opt/../opt/codex",
				}},
			},
		},
	}
	cycle, problems := cycleFromRecord(record, func(receipt issueopscontract.NativeProcessReceipt) (string, issueopscontract.NativeProcessReceipt, error) {
		return operationalhealthcontract.ProcessStatusLive, receipt, nil
	})
	if len(problems) != 0 {
		t.Fatalf("problems = %#v", problems)
	}
	if cycle.LeaseStatus != "active" || cycle.Generation != 3 {
		t.Fatalf("lease projection = %#v", cycle)
	}
	if cycle.HolderHost != "codex" || cycle.HolderSessionID != "session-v1" || cycle.HolderAgentID != "agent-v1" ||
		cycle.HolderPID != 123 || cycle.HolderStartedAt != "2026-07-22T00:00:00Z" || cycle.HolderExecutable != "/opt/../opt/codex" || cycle.HolderProcessStatus != operationalhealthcontract.ProcessStatusLive ||
		cycle.WorktreePath != "/repo.worktrees/69-v1" {
		t.Fatalf("holder/workspace projection = %#v", cycle)
	}
}

func TestCycleFromRecordReportsNativeProcessProbeFailure(t *testing.T) {
	record := issueopscontract.IssueOpsRecord{ID: "io-v1", Repo: "/repo", Branch: "69-v1", Phase: issueopscontract.IssueOpsPhaseImplement, Execution: &issueopscontract.Execution{
		Workspace: issueopscontract.Workspace{Root: "/repo.worktrees/69-v1"},
		Lease: issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusActive, Holder: &issueopscontract.NativeActor{
			Host: "codex", SessionID: "session", SessionProcess: &issueopscontract.NativeProcessReceipt{PID: 123, StartedAt: "2026-07-22T00:00:00Z", Executable: "/opt/codex"},
		}},
	}}
	cycle, problems := cycleFromRecord(record, func(issueopscontract.NativeProcessReceipt) (string, issueopscontract.NativeProcessReceipt, error) {
		return operationalhealthcontract.ProcessStatusUnknown, issueopscontract.NativeProcessReceipt{}, errors.New("probe failed")
	})
	if cycle.HolderProcessStatus != operationalhealthcontract.ProcessStatusUnknown || len(problems) != 1 || problems[0].Code != "issueops_process_probe_failed" {
		t.Fatalf("cycle/problems = %#v / %#v", cycle, problems)
	}
}

func hasProblemCode(problems []operationalhealthcontract.InventoryProblem, code string) bool {
	for _, problem := range problems {
		if problem.Code == code {
			return true
		}
	}
	return false
}

func emptyIssueOpsReader(t *testing.T) IssueOpsReader {
	t.Helper()
	return IssueOpsReader{StateRoot: t.TempDir(), Scan: func(string, func(string, issueopscontract.IssueOpsRecord, error)) error { return nil }, ListLeaseHolders: func(string) ([]issueopscontract.LeaseHolderIndex, error) { return nil, nil }}
}

func TestCollectIssueOpsScansRecordsAndReportsUnreadableRows(t *testing.T) {
	scans := 0
	reader := emptyIssueOpsReader(t)
	reader.Scan = func(string, func(string, issueopscontract.IssueOpsRecord, error)) error { return errors.New("locked") }
	snapshot := corehealth.Snapshot{}
	if (Collector{IssueOps: reader}).collectIssueOps(&snapshot); len(snapshot.Cycles) != 0 || !hasProblemCode(snapshot.InventoryProblems, "issueops_list_failed") {
		t.Fatalf("scan failure cycles=%v problems=%#v", snapshot.Cycles, snapshot.InventoryProblems)
	}
	reader.Scan = func(_ string, visit func(string, issueopscontract.IssueOpsRecord, error)) error {
		scans++
		visit("io-a", issueopscontract.IssueOpsRecord{OK: true, ID: "io-a", Repo: "/repo", Branch: "a"}, nil)
		visit("io-b", issueopscontract.IssueOpsRecord{}, errors.New("invalid state"))
		return nil
	}
	snapshot = corehealth.Snapshot{}
	(Collector{IssueOps: reader}).collectIssueOps(&snapshot)
	if scans != 1 || len(snapshot.Cycles) != 1 || snapshot.Cycles[0].ID != "io-a" {
		t.Fatalf("scans=%d cycles=%v", scans, snapshot.Cycles)
	}
	if len(snapshot.InventoryProblems) != 1 || snapshot.InventoryProblems[0].Code != "issueops_read_failed" || snapshot.InventoryProblems[0].Detail != "could not read IssueOps record io-b" {
		t.Fatalf("problems=%#v", snapshot.InventoryProblems)
	}
}

func TestCycleFromRecordProjectsCleanupFailureAndCompletionPresence(t *testing.T) {
	inspect := func(receipt issueopscontract.NativeProcessReceipt) (string, issueopscontract.NativeProcessReceipt, error) {
		return operationalhealthcontract.ProcessStatusLive, receipt, nil
	}
	now := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name           string
		record         issueopscontract.IssueOpsRecord
		wantCleanup    bool
		wantCompletion bool
	}{
		{name: "none"},
		{name: "finish failure", record: issueopscontract.IssueOpsRecord{CleanupFinishFailure: &issueopscontract.IssueOpsCleanupFinishFailure{}}, wantCleanup: true},
		{name: "abandon failure", record: issueopscontract.IssueOpsRecord{CleanupAbandonFailure: &issueopscontract.IssueOpsCleanupAbandonFailure{}}, wantCleanup: true},
		{name: "completion", record: issueopscontract.IssueOpsRecord{Execution: &issueopscontract.Execution{Completion: &issueopscontract.ExecutionCompletion{}}}, wantCompletion: true},
		{name: "execution without completion", record: issueopscontract.IssueOpsRecord{Execution: &issueopscontract.Execution{}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cycle, _ := cycleFromRecordAt(tc.record, inspect, now)
			if cycle.CleanupFailurePresent != tc.wantCleanup || cycle.CompletionPresent != tc.wantCompletion {
				t.Fatalf("cleanup=%t completion=%t, want cleanup=%t completion=%t", cycle.CleanupFailurePresent, cycle.CompletionPresent, tc.wantCleanup, tc.wantCompletion)
			}
		})
	}
}

func TestCollectMergesIssueOpsAndGitInventory(t *testing.T) {
	reader := emptyIssueOpsReader(t)
	reader.Scan = func(_ string, visit func(string, issueopscontract.IssueOpsRecord, error)) error {
		visit("io-a", issueopscontract.IssueOpsRecord{OK: true, ID: "io-a", Repo: "/repo", Branch: "a", Phase: issueopscontract.IssueOpsPhasePlan}, nil)
		return nil
	}
	git := &scriptedGit{outputs: map[string]string{
		"symbolic-ref": "main\n",
		"rev-parse":    oidA + "\n",
		"status":       "",
		"worktree":     "",
		"for-each-ref": "refs/heads/main\x00" + oidA + "\x00",
		"ls-remote":    oidA + "\trefs/heads/main\n",
	}}
	snapshot := Collector{IssueOps: reader, Git: git}.Collect(context.Background(), t.TempDir())
	if len(snapshot.Cycles) != 1 || snapshot.CanonicalBranch != "main" || snapshot.SourceHead != oidA || len(snapshot.LocalRefs) != 1 || len(snapshot.RemoteRefs) != 1 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if len(snapshot.InventoryProblems) != 0 {
		t.Fatalf("problems = %v", snapshot.InventoryProblems)
	}
}
