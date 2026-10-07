package issueopslease

import (
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
)

func TestValidatePersistedRecordPreservesLeaseRelationshipOrder(t *testing.T) {
	record := leasecontract.Record{Execution: &leasecontract.Execution{
		Mode:      "direct",
		Workspace: leasecontract.Workspace{SourceRoot: "/source", Root: "/worktree", Branch: "branch", BaseHead: strings.Repeat("a", 40), Driver: "git", LinkedAt: "2026-08-03T00:00:00Z"},
		Lease:     leasecontract.Lease{Generation: 1, Status: "released"},
		Selection: leaseSelectionFixture("direct"),
	}}
	if err := ValidatePersistedRecord(record); err != nil {
		t.Fatalf("valid released record: %v", err)
	}
	record.Execution.Workspace.Root = "/source"
	record.Execution.Lease.Generation = 0
	if err := ValidatePersistedRecord(record); err == nil || err.Error() != "canonical worktree must be isolated from source_root" {
		t.Fatalf("workspace error must precede lease error: %v", err)
	}
	record.Execution.Workspace.Root = "/worktree"
	if err := ValidatePersistedRecord(record); err == nil || err.Error() != "lease generation must start at 1" {
		t.Fatalf("lease error: %v", err)
	}
}
