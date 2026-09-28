package issueopspreparation

import (
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	preparationdomain "issueops/internal/domain/issueopspreparation"
)

func TestApplyDirectCommitBindsCurrentRecordAndRejectsPreparedState(t *testing.T) {
	record := leasecontract.Record{ID: "io-1", IssueURL: "https://github.com/acme/repo/issues/21"}
	command := preparationcontract.Command{ID: record.ID, Mode: preparationcontract.ModeDirect, DirectReason: "planned recovery", Actor: leasecontract.Actor{Host: "codex", SessionID: "s"}, Confirm: true}
	decision, err := preparationdomain.Decide(preparationdomain.DecisionInput{Command: command})
	if err != nil {
		t.Fatal(err)
	}
	commit := DirectCommit{Command: command, Workspace: preparationcontract.WorkspaceReceipt{SourceRoot: "/source", Root: "/worktree", Branch: "b", BaseHead: "base"}, RequestedMode: preparationcontract.ModeDirect, LinkedAt: "linked", ClaimedAt: "claimed", Selection: leasecontract.Selection{RequestedMode: preparationcontract.ModeDirect, ResolvedMode: preparationcontract.ModeDirect, ReadinessFingerprint: decision.ReadinessFingerprint, SelectedAt: "linked", ExplicitDirectReason: "planned recovery"}}
	commit.ArtifactDir = "artifact-dir"
	after, result, err := ApplyDirectCommit(record, commit)
	if err != nil {
		t.Fatal(err)
	}
	if record.Execution != nil || after.Execution == nil || after.Execution.Lease.Holder.SessionID != "s" || after.Execution.Workspace.Root != "/worktree" || after.Execution.Workspace.ArtifactDir != "artifact-dir" || result.Execution == nil || result.Execution.Lease.ClaimedAt != "claimed" {
		t.Fatalf("record=%+v after=%+v result=%+v", record, after, result)
	}
	if _, _, err := ApplyDirectCommit(after, commit); err == nil || !strings.Contains(err.Error(), "already prepared") {
		t.Fatalf("prepared error=%v", err)
	}
}
