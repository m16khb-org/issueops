package issueopsretention

import (
	"testing"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestIsPrunableRequiresResolvedIssueCreateIntent(t *testing.T) {
	cutoff := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	record := issueopscontract.IssueOpsRecord{
		Phase:     issueopscontract.IssueOpsPhaseDone,
		UpdatedAt: cutoff.Add(-time.Hour).Format(time.RFC3339Nano),
		IssueCreateIntent: &issueopscontract.IssueOpsIssueCreateIntent{
			Status: issueopscontract.IssueCreateIntentInvokedUnknown,
		},
	}
	if IsPrunable(record, cutoff) {
		t.Fatal("unresolved issue-create intent must retain reconciliation authority")
	}
	record.IssueCreateIntent.Status = issueopscontract.IssueCreateIntentCompleted
	if !IsPrunable(record, cutoff) {
		t.Fatal("completed issue-create intent may be pruned after the cutoff")
	}
}

func TestPruneRetainsFinishAttemptRegardlessOfAge(t *testing.T) {
	cutoff := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	record := issueopscontract.IssueOpsRecord{Phase: issueopscontract.IssueOpsPhaseDone, UpdatedAt: "2000-01-01T00:00:00Z", CleanupAttempt: &issueopscontract.IssueOpsCleanupAttempt{Operation: "finish", Token: "pending", StartedAt: "2000-01-01T00:00:00Z"}}
	if IsPrunable(record, cutoff) {
		t.Fatal("retention discarded finish recovery authority")
	}
	record.CleanupAttempt = nil
	if !IsPrunable(record, cutoff) {
		t.Fatal("drained old record cannot be pruned")
	}
}
