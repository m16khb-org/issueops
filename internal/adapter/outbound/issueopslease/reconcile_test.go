package issueopslease

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	leaseapp "issueops/internal/application/issueopslease"
	leasecontract "issueops/internal/contract/issueopslease"
	"issueops/internal/port"
)

type reconcileEffectsFake struct {
	state          ReconcileEffectState
	clearedIntent  bool
	clearIntentErr error
}

func (f *reconcileEffectsFake) Canonicalize(context.Context, string) (ReconcileEffectState, error) {
	return f.state, nil
}
func (f *reconcileEffectsFake) ApplyReceipt(context.Context, ReconcileEffectState, leasecontract.ReconcileStageReceipt) (ReconcileEffectState, error) {
	return f.state, nil
}

func TestReconcileRepositoryPreservesRawCASState(t *testing.T) {
	effects := &reconcileEffectsFake{state: ReconcileEffectState{
		Record:    leasecontract.Record{ID: "io-1", Execution: &leasecontract.Execution{}},
		RecordRaw: []byte("record-raw"), IntentRaw: []byte("intent-raw"), OperationID: "op-1",
		Stage: "run_bind", InvocationState: "unknown", InvocationAttempts: 1, Pending: true,
	}}
	state, err := NewReconcileRepository(nil, effects).Canonicalize(context.Background(), "io-1")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(state.RecordRaw, effects.state.RecordRaw) || !bytes.Equal(state.IntentRaw, effects.state.IntentRaw) {
		t.Fatalf("state = %#v", state)
	}
}

func TestReconcileRepositoryLatestReadsOutboundStore(t *testing.T) {
	_, store := newResumeRepositoryStore(t, resumeRepositoryRecord(t, 4))
	repository := NewReconcileRepository(store, &reconcileEffectsFake{})
	record, err := repository.Latest(context.Background(), "io-resume-repository")
	if err != nil {
		t.Fatal(err)
	}
	if record.Execution == nil || record.Execution.Lease.Generation != 4 {
		t.Fatalf("latest record=%+v", record)
	}
}

func TestReconcileRepositoryMarkInvokingUsesRawCAS(t *testing.T) {
	_, state, store := seededResumeIntent(t)
	repository := NewReconcileRepository(store, nil)
	intent := leaseapp.ReconcileIntentState{
		Progress:    leaseapp.ReconcileProgress{Record: state.Progress.Record.Stable, Pending: true, NextStage: state.Stage},
		OperationID: state.OperationID, Stage: state.Stage, InvocationState: state.InvocationState,
		InvocationAttempts: state.InvocationAttempts, RecordRaw: state.RecordRaw, IntentRaw: state.IntentRaw,
	}
	next, err := repository.MarkInvoking(context.Background(), intent)
	if err != nil {
		t.Fatal(err)
	}
	if next.InvocationState != "unknown" || next.InvocationAttempts != 1 {
		t.Fatalf("next=%+v", next)
	}
	if _, err := repository.MarkInvoking(context.Background(), intent); err == nil {
		t.Fatal("stale raw intent was accepted")
	}
}

func TestReconcileRepositoryRecordFailureUsesRawCAS(t *testing.T) {
	_, state, store := seededResumeIntent(t)
	repository := NewReconcileRepository(store, nil)
	repository.now = func() time.Time { return time.Date(2026, time.July, 31, 3, 5, 0, 0, time.UTC) }
	repository.redact = func(string) string { return "redacted failure" }
	intent := leaseapp.ReconcileIntentState{
		Progress:    leaseapp.ReconcileProgress{Record: state.Progress.Record.Stable, Pending: true, NextStage: state.Stage},
		OperationID: state.OperationID, Stage: state.Stage, InvocationState: state.InvocationState,
		InvocationAttempts: state.InvocationAttempts, RecordRaw: state.RecordRaw, IntentRaw: state.IntentRaw,
	}
	cause := &port.OrcaError{CallPhase: "terminal_send", DispatchRequestID: "11111111-1111-4111-8111-111111111111"}
	if err := repository.RecordFailure(context.Background(), intent, "unknown", cause); err != nil {
		t.Fatal(err)
	}
	data, ok, err := store.Get(recordBucket, intent.Progress.Record.ID)
	if err != nil || !ok {
		t.Fatalf("record: present=%v err=%v", ok, err)
	}
	record, err := decodeLeaseRecord(intent.Progress.Record.ID, data)
	if err != nil {
		t.Fatal(err)
	}
	if got := record.Execution.Failure; got == nil || got.Message != "redacted failure" || got.At != "2026-07-31T03:05:00Z" {
		t.Fatalf("failure=%+v", got)
	}
	if err := repository.RecordFailure(context.Background(), intent, "unknown", cause); err == nil {
		t.Fatal("stale raw snapshot was accepted")
	}
}

func TestReconcileStageExecutorPreservesAttemptDisclosure(t *testing.T) {
	wantErr := errors.New("transport")
	executor := NewReconcileStageExecutor(
		func(context.Context, leaseapp.ReconcileIntentState) (leasecontract.ReconcileStageInventory, bool, error) {
			return leasecontract.ReconcileStageInventory{}, true, wantErr
		},
		nil,
	)
	_, attempted, err := executor.Inspect(context.Background(), leaseapp.ReconcileIntentState{})
	if !attempted || !errors.Is(err, wantErr) {
		t.Fatalf("attempted=%t err=%v", attempted, err)
	}
}

func (f *reconcileEffectsFake) ClearIntent(_ context.Context, state ReconcileEffectState, _ error) (ReconcileEffectState, error) {
	f.clearedIntent = true
	state.Pending = false
	return state, f.clearIntentErr
}
