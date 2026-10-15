package issueopspreparation

import (
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func TestApplyDirectCommitBindsCurrentRecordAndRejectsPreparedState(t *testing.T) {
	record := leasecontract.Record{ID: "io-1", IssueURL: "https://github.com/acme/repo/issues/21"}
	command := preparationcontract.Command{ID: record.ID, Actor: leasecontract.Actor{Host: "codex", SessionID: "s"}, Confirm: true}
	commit := DirectCommit{Command: command, Workspace: preparationcontract.WorkspaceReceipt{SourceRoot: "/source", Root: "/worktree", Branch: "b", BaseHead: "base"}, LinkedAt: "linked", ClaimedAt: "claimed"}
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
