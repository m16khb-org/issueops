package issueopspreparation

import (
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
)

func TestPreparationCloneIsolatesEveryExecutionPointer(t *testing.T) {
	holder := &leasecontract.Actor{Host: "codex", SessionID: "s"}
	execution := &leasecontract.Execution{
		Lease:   leasecontract.Lease{Holder: holder},
		Pending: &leasecontract.ExternalIntent{OperationID: "op"},
		Failure: &leasecontract.FailureDetail{Code: "boom"},
	}
	cloned := Result{Execution: execution}.Clone()
	cloned.Execution.Pending.OperationID = "changed"
	cloned.Execution.Failure.Code = "changed"
	cloned.Execution.Lease.Holder.Host = "claude"
	if execution.Pending.OperationID != "op" || execution.Failure.Code != "boom" || execution.Lease.Holder.Host != "codex" {
		t.Fatalf("clone shares pointers with its source: %+v", execution)
	}
	// Absent optional parts stay absent rather than becoming empty values.
	bare := Result{Execution: &leasecontract.Execution{}}.Clone()
	e := bare.Execution
	if e.Pending != nil || e.Failure != nil || e.Completion != nil ||
		e.CompletionHistory != nil || e.SyncBaseResolution != nil || e.SyncBaseEvents != nil || e.Lease.Holder != nil {
		t.Fatalf("bare clone materialized optional fields: %+v", e)
	}
	if got := (Result{}).Clone(); got.Execution != nil {
		t.Fatalf("nil execution cloned into %+v", got.Execution)
	}
}

func TestPreparationCloneKeepsNilVersusEmptyStrings(t *testing.T) {
	execution := &leasecontract.Execution{
		Completion: &leasecontract.Completion{},
		CompletionHistory: []leasecontract.CompletionHistoryEntry{
			{Completion: leasecontract.Completion{Verification: []string{}}},
		},
	}
	cloned := Result{Execution: execution}.Clone().Execution
	if cloned.Completion.Verification != nil {
		t.Fatalf("nil verification became %#v", cloned.Completion.Verification)
	}
	if v := cloned.CompletionHistory[0].Completion.Verification; v == nil || len(v) != 0 {
		t.Fatalf("empty verification became %#v", v)
	}
}
