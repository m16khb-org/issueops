package issueopspreparation

import (
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func TestApplyOrcaReceiptAdvancesWorktreeAndRejectsMissingTerminal(t *testing.T) {
	state := IntentState{
		Intent:         preparationcontract.Intent{OperationID: "op", Generation: 1, Stage: preparationcontract.IntentStageWorktree, StartedAt: "started"},
		Snapshot:       preparationcontract.Snapshot{Record: leasecontract.Record{ID: "id", Execution: &leasecontract.Execution{Pending: &leasecontract.ExternalIntent{}}}},
		OwnerArtifacts: preparationcontract.OwnerArtifacts{PlanPath: "plan", ClaimTokenSHA256: strings.Repeat("a", 64)},
	}
	receipt := preparationcontract.IntentReceipt{Workspace: &preparationcontract.OrcaWorkspaceReceipt{Workspace: preparationcontract.WorkspaceReceipt{Root: "/worktree", Driver: "orca"}}}
	decision, err := ApplyOrcaReceipt(state, receipt, "artifact-dir", nil)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Complete || decision.Intent.Stage != preparationcontract.IntentStageTerminal || decision.Record.WorktreePath != "/worktree" || decision.Record.Execution.Pending.Kind != "owner_launch" || decision.Record.Execution.Workspace.ArtifactDir != "artifact-dir" {
		t.Fatalf("decision=%+v", decision)
	}
	if state.Snapshot.Record.Execution.Pending.Kind != "" || state.Snapshot.Record.Execution.Workspace.Root != "" {
		t.Fatalf("input state changed before CAS: %+v", state.Snapshot.Record.Execution)
	}
	state.Intent = decision.Intent
	state.Snapshot.Record = decision.Record
	if _, err := ApplyOrcaReceipt(state, preparationcontract.IntentReceipt{}, "artifact-dir", nil); err == nil || !strings.Contains(err.Error(), "terminal candidate is incomplete") {
		t.Fatalf("terminal error=%v", err)
	}
}
