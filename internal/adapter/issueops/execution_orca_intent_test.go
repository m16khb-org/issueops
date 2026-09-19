package issueops

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"issueops/internal/contract/issueops"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	"issueops/internal/port"
)

func TestOrcaIntentWorktreeReceiptPersistsPlanBeforeNextIntent(t *testing.T) {
	stateRoot, record := orcaPrepareRecord(t)
	const plan = "# Intent owner plan\n"
	if _, err := stageIssueOpsArtifactForTest(stateRoot, record.ID, "plan", []byte(plan)); err != nil {
		t.Fatal(err)
	}
	worktree := issueOpsWorktreePathForTest(record.Repo, record.Branch)
	workspace := port.ExecutionWorkspaceRequest{
		LifecycleID: record.ID, SourceRoot: record.Repo, Root: worktree, Branch: record.Branch,
		BaseBranch: record.BranchPrepare.BaseBranch, BaseHead: record.BranchPrepare.BaseSHA, Confirm: true,
	}
	probe := port.ExecutionOrcaProbeRequest{
		Repo: record.Repo, Host: "codex", Model: "gpt-5.6-terra", Effort: "xhigh",
		Provider: "github", Issue: 16, Marker: "readiness-marker",
	}
	issueBody := "## Acceptance\n- AC-01 persist plan\n\n## Verification\n```bash\ngo test ./... -count=1\n```\n"
	snapshot := executionOwnerSnapshot{issue: executionOwnerIssue{
		URL: record.IssueURL, Body: issueBody, BodySHA256: digestExecutionOwnerBytes([]byte(issueBody)),
	}}
	prepared, intent, err := beginOrcaExecutionIntent(
		stateRoot, record, workspace, probe,
		ExecutionPrepareRequest{ID: record.ID, Mode: "orca", OwnerHost: "codex", OwnerModel: "gpt-5.6-terra", OwnerEffort: "xhigh"},
		snapshot, func() time.Time { return time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	receipt := port.ExecutionOrcaIntentReceipt{Workspace: &port.ExecutionOrcaWorkspaceReceipt{
		Workspace: port.ExecutionWorkspaceReceipt{
			SourceRoot: record.Repo, Root: worktree, Branch: record.Branch,
			BaseHead: record.BranchPrepare.BaseSHA, Driver: "orca", Exists: true,
		},
		RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", WorktreeInstanceID: "instance",
	}}
	readIssue := func(_ context.Context, _ string, request port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
		return port.ExecutionIssueSnapshot{URL: request.URL, Body: issueBody}, nil
	}

	advanced, next, err := advanceOrcaIntentReceipt(context.Background(), stateRoot, prepared, intent, receipt, readIssue, nil)
	if err != nil {
		t.Fatal(err)
	}
	// #482: linked issue 16 seals under the issue folder, recorded in workspace.artifact_dir.
	wantPath := filepath.Join(worktree, ".issueops", "issues", "16", "artifact", "plan.md")
	if advanced.PlanPath != wantPath || next.Stage != "terminal_create" {
		t.Fatalf("advanced plan=%q stage=%q want plan=%q terminal_create", advanced.PlanPath, next.Stage, wantPath)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.PlanPath != wantPath || persisted.Execution.Pending == nil || persisted.Execution.Pending.Kind != "owner_launch" {
		t.Fatalf("persisted plan/intent=%q %#v", persisted.PlanPath, persisted.Execution.Pending)
	}
	content, err := os.ReadFile(wantPath)
	if err != nil || string(content) != plan {
		t.Fatalf("materialized plan=%q err=%v", content, err)
	}
	if advanced.Execution.Lease.Status != issueops.LeaseStatusReleased || advanced.Execution.Orca != nil {
		t.Fatalf("worktree receipt advanced lease or owner binding: %#v", advanced.Execution)
	}
}

func TestRecordOrcaIntentTerminalSendFailurePreservesDispatchAndPromptRequestIDs(t *testing.T) {
	stateRoot, record, payload := resumeIntentFixture(t, "github", 16)
	var err error
	for payload.Stage != preparationcontract.IntentStageDispatch {
		receipt := port.ExecutionOrcaIntentReceipt{}
		switch payload.Stage {
		case preparationcontract.IntentStageTerminal:
			receipt.TerminalPTYID = "pty-next"
		case preparationcontract.IntentStageRun:
			receipt.RunID = "run-next"
		case preparationcontract.IntentStageRunBind:
			receipt.RunID = "run-next"
			receipt.RunBound = true
		case preparationcontract.IntentStageTask:
			receipt.TaskID = "task-next"
		default:
			t.Fatalf("unexpected stage before dispatch: %s", payload.Stage)
		}
		record, payload, err = advanceOrcaIntentReceipt(context.Background(), stateRoot, record, payload, receipt, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	state, err := ReadExecutionResumeIntent(stateRoot, record.ID, payload.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	cause := &port.OrcaError{
		Code: "operation_unknown", Invoked: true, CallPhase: "terminal_send",
		DispatchRequestID:      "11111111-1111-4111-8111-111111111111",
		OrchestrationRequestID: "22222222-2222-4222-8222-222222222222",
	}
	if err := RecordExecutionResumeIntentFailure(stateRoot, state, orcaIntentUnknown, cause, nil); err != nil {
		t.Fatal(err)
	}
	updated, err := ReadExecutionResumeIntent(stateRoot, record.ID, payload.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	updatedPayload, err := executionResumeIntentPayload(updated)
	if err != nil {
		t.Fatal(err)
	}
	if updatedPayload.OrcaRequestID != "11111111-1111-4111-8111-111111111111" || updatedPayload.OrcaPromptRequestID != "22222222-2222-4222-8222-222222222222" {
		t.Fatalf("durable IDs not preserved separately: dispatch=%q prompt=%q", updatedPayload.OrcaRequestID, updatedPayload.OrcaPromptRequestID)
	}
}
