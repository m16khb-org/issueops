package issueopsexecution

import (
	"issueops/internal/contract/issueops"
	"testing"
)

func TestWithExecutionIssueSnapshotSource(t *testing.T) {
	if got := withExecutionIssueSnapshotSource(issueops.ExecutionPrepareResult{}, ""); got.(issueops.ExecutionPrepareResult).IssueSnapshotSource != "" {
		t.Fatal("empty source must not decorate")
	}
	prepared := withExecutionIssueSnapshotSource(issueops.ExecutionPrepareResult{}, "glab_mcp").(issueops.ExecutionPrepareResult)
	if prepared.IssueSnapshotSource != "glab_mcp" {
		t.Fatalf("prepare decoration wrong: %+v", prepared)
	}
	exe := withExecutionIssueSnapshotSource(issueops.ExecutionResult{}, "glab_cli").(issueops.ExecutionResult)
	if exe.IssueSnapshotSource != "glab_cli" {
		t.Fatalf("execution decoration wrong: %+v", exe)
	}
	replace := withExecutionIssueSnapshotSource(issueops.ExecutionReplaceResult{}, "s").(issueops.ExecutionReplaceResult)
	if replace.IssueSnapshotSource != "s" {
		t.Fatalf("replace decoration wrong: %+v", replace)
	}
	reconcile := withExecutionIssueSnapshotSource(issueops.ExecutionReconcileResult{}, "s").(issueops.ExecutionReconcileResult)
	if reconcile.IssueSnapshotSource != "s" {
		t.Fatalf("reconcile decoration wrong: %+v", reconcile)
	}
	if got := withExecutionIssueSnapshotSource(42, "s"); got != 42 {
		t.Fatal("unknown types must pass through unchanged")
	}
}
