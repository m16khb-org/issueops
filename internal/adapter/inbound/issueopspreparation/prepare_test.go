package issueopspreparation

import (
	"context"
	"errors"
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"

	leasecontract "issueops/internal/contract/issueopslease"

	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func TestHandlerMapsEveryRequestAndResultField(t *testing.T) {
	process := model.NativeProcessReceipt{PID: 199, StartedAt: "2026-08-02T01:02:03.123456789Z", Executable: "/usr/local/bin/codex"}
	request := model.ExecutionPrepareRequest{
		ID:    "io-199",
		Actor: model.NativeActor{Host: "codex", SessionID: "session-199", AgentID: "agent-199", SessionProcess: &process},
		CWD:   "/repo", Confirm: true,
	}
	execution := fullContractExecution()
	service := &serviceFake{result: preparationcontract.Result{
		OK: true, ID: request.ID, Preview: true,
		Workspace: execution.Workspace, Execution: &execution,
		ClaimTokenPath: "/tokens/199", NextCommand: "issueops next",
	}}

	got, err := NewHandler(service)(context.Background(), "/state", request)
	if err != nil {
		t.Fatal(err)
	}
	wantCommand := preparationcontract.Command{
		ID: request.ID,
		Actor: leasecontract.Actor{
			Host: request.Actor.Host, SessionID: request.Actor.SessionID, AgentID: request.Actor.AgentID,
			SessionProcess: &leasecontract.ProcessReceipt{PID: process.PID, StartedAt: process.StartedAt, Executable: process.Executable},
		},
		CWD: request.CWD, Confirm: true,
	}
	if !reflect.DeepEqual(service.command, wantCommand) {
		t.Fatalf("command=%#v want=%#v", service.command, wantCommand)
	}
	want := model.ExecutionPrepareResult{
		OK: true, ID: request.ID, Preview: true,
		Workspace: coreWorkspace(execution.Workspace), Execution: coreExecution(&execution),
		ClaimTokenPath: "/tokens/199", NextCommand: "issueops next",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("result=%#v want=%#v", got, want)
	}
	if len(got.Execution.CompletionHistory) != 1 || got.Execution.CompletionHistory[0].Completion.Verification[0] != "old history verification" {
		t.Fatalf("completion history=%+v", got.Execution.CompletionHistory)
	}

	request.Actor.SessionProcess.PID = 999
	if !reflect.DeepEqual(service.command, wantCommand) {
		t.Fatalf("mapped command aliases public request: %#v", service.command)
	}
}

func TestHandlerPreservesResultWithServiceError(t *testing.T) {
	cause := errors.New("pending external intent")
	service := &serviceFake{result: preparationcontract.Result{ID: "io-199", NextCommand: "reconcile"}, err: cause}

	got, err := NewHandler(service)(context.Background(), "/state", model.ExecutionPrepareRequest{ID: "io-199"})
	if err != cause || got.ID != "io-199" || got.NextCommand != "reconcile" {
		t.Fatalf("result=%#v err=%v", got, err)
	}
}

func TestHandlerFailsClosedWithoutService(t *testing.T) {
	got, err := NewHandler(nil)(context.Background(), "/state", model.ExecutionPrepareRequest{ID: "io-199"})
	if !errors.Is(err, model.ErrPrepareHandlerUnavailable) || got != (model.ExecutionPrepareResult{ID: "io-199"}) {
		t.Fatalf("result=%#v err=%v", got, err)
	}
}

type serviceFake struct {
	command preparationcontract.Command
	result  preparationcontract.Result
	err     error
}

func (fake *serviceFake) Prepare(_ context.Context, command preparationcontract.Command) (preparationcontract.Result, error) {
	fake.command = command.Clone()
	return fake.result.Clone(), fake.err
}

func fullContractExecution() leasecontract.Execution {
	return leasecontract.Execution{
		Workspace: leasecontract.Workspace{SourceRoot: "/source", Root: "/worktree", Branch: "199-vertical", BaseHead: "base", ParentWorktree: "/parent", LinkedAt: "2026-08-02T02:03:04Z"},
		Lease: leasecontract.Lease{
			Generation: 3, Status: "active",
			Holder:           &leasecontract.Actor{Host: "claude", SessionID: "owner", AgentID: "agent", SessionProcess: &leasecontract.ProcessReceipt{PID: 200, StartedAt: "2026-08-02T02:03:05Z", Executable: "/bin/claude"}},
			ClaimTokenSHA256: "claim-sha", ClaimedAt: "claimed", ReleasedAt: "released", ReplacedAt: "replaced", ReplacementReason: "reason",
		},
		Pending:    &leasecontract.ExternalIntent{OperationID: "operation", Kind: "remote_pr_create", Marker: "marker", StartedAt: "started"},
		Completion: &leasecontract.Completion{FinalHead: "head", VerificationReportPath: "/report", Verification: []string{"go test ./..."}, RemoteArtifactURL: "https://example.test/pr/199", CompletedAt: "completed"},
		CompletionHistory: []leasecontract.CompletionHistoryEntry{{
			Generation: 2, Completion: leasecontract.Completion{Verification: []string{"old history verification"}}, Reason: "reseed", ReopenedAt: "reopened",
		}},
		Failure:        &leasecontract.FailureDetail{OperationID: "operation", Code: "failed", Message: "redacted", At: "failed-at"},
		SyncBaseEvents: []leasecontract.SyncBaseEvent{{Mode: "merge", BaseBranch: "main", BaseOID: "base", MergeCommit: "merge", ConflictFiles: 2, Actor: "codex", At: "synced"}},
	}
}

func TestHandlerMapsEachOptionalExecutionSectionToExpectedValues(t *testing.T) {
	execution := fullContractExecution()
	service := &serviceFake{result: preparationcontract.Result{ID: "io-199", Execution: &execution}}

	got, err := NewHandler(service)(context.Background(), "/state", model.ExecutionPrepareRequest{ID: "io-199"})
	if err != nil {
		t.Fatal(err)
	}
	exec := got.Execution
	if exec == nil {
		t.Fatal("execution missing")
	}
	holder := exec.Lease.Holder
	if holder == nil || holder.Host != "claude" || holder.SessionID != "owner" || holder.AgentID != "agent" {
		t.Fatalf("holder=%+v", holder)
	}
	if holder.SessionProcess == nil || holder.SessionProcess.PID != 200 || holder.SessionProcess.Executable != "/bin/claude" {
		t.Fatalf("holder process=%+v", holder.SessionProcess)
	}
	if exec.Pending == nil || exec.Pending.OperationID != "operation" || exec.Pending.Kind != "remote_pr_create" {
		t.Fatalf("pending=%+v", exec.Pending)
	}
	if exec.Completion == nil || exec.Completion.FinalHead != "head" || len(exec.Completion.Verification) != 1 {
		t.Fatalf("completion=%+v", exec.Completion)
	}
	if exec.Failure == nil || exec.Failure.Code != "failed" || exec.Failure.Message != "redacted" {
		t.Fatalf("failure=%+v", exec.Failure)
	}
	if len(exec.SyncBaseEvents) != 1 || exec.SyncBaseEvents[0].BaseBranch != "main" || exec.SyncBaseEvents[0].ConflictFiles != 2 {
		t.Fatalf("sync base events=%+v", exec.SyncBaseEvents)
	}
}
