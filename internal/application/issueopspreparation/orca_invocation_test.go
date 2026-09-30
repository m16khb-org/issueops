package issueopspreparation

import (
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func TestOrcaInvocationAndFailureTransition(t *testing.T) {
	intent := preparationcontract.Intent{InvocationState: preparationcontract.InvocationNotInvoked, InvocationAttempts: 1, OperationID: "op"}
	marked := MarkOrcaInvoking(intent)
	if marked.InvocationState != preparationcontract.InvocationUnknown || marked.InvocationAttempts != 2 || intent.InvocationAttempts != 1 {
		t.Fatalf("marked=%+v original=%+v", marked, intent)
	}
	state := IntentState{Intent: marked, Snapshot: preparationcontract.Snapshot{Record: leasecontract.Record{ID: "id", Execution: &leasecontract.Execution{}}}, FailureAt: "now"}
	record, failed, err := ApplyOrcaFailure(state, preparationcontract.InvocationUnknown, func() string { return "redacted" })
	if err != nil {
		t.Fatal(err)
	}
	if failed.InvocationState != preparationcontract.InvocationUnknown || record.Execution.Failure == nil || record.Execution.Failure.Message != "redacted" || record.Execution.Failure.OperationID != "op" {
		t.Fatalf("record=%+v failed=%+v", record, failed)
	}
	if state.Snapshot.Record.Execution.Failure != nil {
		t.Fatalf("input state changed before CAS: %+v", state.Snapshot.Record.Execution)
	}
	state.FailureAt = ""
	if _, _, err := ApplyOrcaFailure(state, preparationcontract.InvocationUnknown, func() string { t.Fatal("diagnostic observed before timestamp validation"); return "" }); err == nil {
		t.Fatal("missing failure timestamp accepted")
	}
}

func TestApplyOrcaNoResourceClearsPendingWithoutMutatingSnapshot(t *testing.T) {
	state := IntentState{
		Intent: preparationcontract.Intent{OperationID: "op"}, FailureAt: "now",
		Snapshot: preparationcontract.Snapshot{Record: leasecontract.Record{ID: "id", Execution: &leasecontract.Execution{Pending: &leasecontract.ExternalIntent{OperationID: "op"}}}},
	}
	record, err := ApplyOrcaNoResource(state, func() string { return "redacted" })
	if err != nil {
		t.Fatal(err)
	}
	if record.Execution.Pending != nil || record.Execution.Failure == nil || record.Execution.Failure.Code != "external_operation_left_no_resource" || record.Execution.Failure.Message != "redacted" {
		t.Fatalf("record=%+v", record)
	}
	if state.Snapshot.Record.Execution.Pending == nil || state.Snapshot.Record.Execution.Failure != nil {
		t.Fatalf("snapshot mutated=%+v", state.Snapshot.Record)
	}
	state.FailureAt = ""
	if _, err := ApplyOrcaNoResource(state, func() string { t.Fatal("diagnostic observed before timestamp validation"); return "" }); err == nil {
		t.Fatal("missing timestamp accepted")
	}
}
