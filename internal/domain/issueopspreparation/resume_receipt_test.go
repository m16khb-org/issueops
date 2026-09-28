package issueopspreparation

import (
	"strings"
	"testing"

	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func TestApplyResumeReceiptAdvancesPendingAndPreservesLease(t *testing.T) {
	record, intent := resumeReceiptFixture(t)
	next, err := ApplyResumeReceipt(record, intent, preparationcontract.IntentReceipt{TerminalPTYID: " pty-new "}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if next.Complete || next.Intent.Stage != preparationcontract.IntentStageRun || next.Intent.TerminalPTYID != "pty-new" || next.Record.Execution.Pending.Kind != PendingKind(preparationcontract.IntentStageRun) {
		t.Fatalf("next=%+v", next)
	}
	if !leasesEqual(next.Record.Execution.Lease, record.Execution.Lease) || record.Execution.Pending.Kind != PendingKind(preparationcontract.IntentStageTerminal) {
		t.Fatalf("resume lease or source record changed: next=%+v source=%+v", next.Record.Execution, record.Execution)
	}
	if _, err := ApplyResumeReceipt(record, intent, preparationcontract.IntentReceipt{}, nil); err == nil || !strings.Contains(err.Error(), "terminal candidate is incomplete") {
		t.Fatalf("missing terminal receipt error=%v", err)
	}
}

func TestApplyResumeReceiptCompletesWithNewBindingAndOriginalLease(t *testing.T) {
	record, intent := resumeReceiptFixture(t)
	intent.Stage = preparationcontract.IntentStageDispatch
	intent.TerminalPTYID, intent.RunID, intent.RunBound, intent.TaskID = "pty-new", "run-new", true, "task-new"
	record.Execution.Pending.Kind = PendingKind(intent.Stage)
	lease := record.Execution.Lease
	next, err := ApplyResumeReceipt(record, intent, preparationcontract.IntentReceipt{DispatchID: "dispatch-new", RequestID: "request-new"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !next.Complete || next.Record.Execution.Pending != nil || next.Record.Execution.Orca.RuntimeID != "new-runtime" || next.Record.Execution.Orca.DispatchID != "dispatch-new" || next.Record.Execution.Orca.RunID != "run-new" || next.Record.Execution.Orca.TerminalPTYID != "pty-new" {
		t.Fatalf("completed resume=%+v", next)
	}
	if !leasesEqual(next.Record.Execution.Lease, lease) || record.Execution.Pending == nil || record.Execution.Orca.RuntimeID != "old-runtime" {
		t.Fatalf("resume lease or source record changed: next=%+v source=%+v", next.Record.Execution, record.Execution)
	}
}

func resumeReceiptFixture(t *testing.T) (preparationcontract.Record, preparationcontract.Intent) {
	t.Helper()
	record := preparationcontract.Record{
		ID: "io-1", Repo: "/repo", IssueURL: "https://github.com/example/repo/issues/193",
		BranchPrepare: []byte(`{"provider":"github","issue_url":"https://github.com/example/repo/issues/193","link_verified":true}`),
		Execution: &preparationcontract.Execution{
			Mode: "orca", Workspace: preparationcontract.Workspace{SourceRoot: "/repo", Root: "/repo.worktrees/193-fix", Branch: "193-fix", BaseHead: "base"},
			Lease: preparationcontract.Lease{Generation: 4, Status: "claimable", ClaimTokenSHA256: strings.Repeat("b", 64)},
			Orca: &preparationcontract.OrcaBinding{
				RuntimeID: "old-runtime", RepoID: "repo-id", WorktreeID: "tree-id", OwnerHost: "codex", OwnerModel: "model", OwnerEffort: "high", TaskID: "task-old", DispatchID: "dispatch-old", LeaseGeneration: 4,
			},
		},
	}
	intent, err := BuildResumeIntent(ResumeIntentInput{
		Record: record, Workspace: preparationcontract.WorkspaceRequest{LifecycleID: record.ID, SourceRoot: "/repo", Root: "/repo.worktrees/193-fix", Branch: "193-fix", BaseHead: "base", Confirm: true},
		Artifacts: preparationcontract.ResumeArtifacts{IssueBodySHA256: strings.Repeat("a", 64), OwnerPromptPath: "/prompt", OwnerPromptSHA256: strings.Repeat("c", 64), ContextPacketPath: "/packet", ContextPacketSHA256: strings.Repeat("d", 64)},
		RuntimeID: "new-runtime", OperationID: strings.Repeat("e", 32), StartedAt: "2026-07-31T03:15:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	return ApplyResumeIntent(record, intent), intent
}
