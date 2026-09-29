package issueopsapp

import (
	"os"
	"path/filepath"
	"testing"

	auditadapter "issueops/internal/adapter/audit"
	issueopsadapter "issueops/internal/adapter/issueops"
)

func TestManualHandoffServiceKeepsCapturedStateAndSnapshot(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
	stateRoot := issueopsadapter.IssueOpsStateRoot()
	record := seedReleasedDirectHandoffRecord(t, stateRoot)
	service := newHandoffDeliveryService(stateRoot)
	observation := manualCmuxHandoffObservation(record.ID, record.Execution.Lease.Generation)
	otherState := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", otherState)
	if _, err := service.ObserveManual(observation); err != nil {
		t.Fatal(err)
	}
	// The cmux path already has a fenced snapshot. It must not re-read the
	// now advanced stored record while validating that supplied snapshot.
	advanced := record
	copied := *record.Execution
	advanced.Execution = &copied
	advanced.Execution.Lease.Generation++
	if _, err := issueopsadapter.WriteIssueOps(stateRoot, advanced); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ObserveManual(observation); err == nil {
		t.Fatal("stale stored generation accepted")
	}
	if _, err := service.ObserveManualSnapshot(record, observation); err != nil {
		t.Fatal(err)
	}
	records, err := auditadapter.ReadHandoffDeliveryAuditObservationsAt(stateRoot)
	if err != nil || len(records) != 2 {
		t.Fatalf("records=%d err=%v", len(records), err)
	}
	if _, err := os.Stat(filepath.Join(issueopsadapter.IssueOpsStateRoot(), "audit", "handoff-delivery.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("ambient state was written: %v", err)
	}
}
