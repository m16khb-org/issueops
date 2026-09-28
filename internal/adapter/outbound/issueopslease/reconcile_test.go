package issueopslease

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	leaseapp "issueops/internal/application/issueopslease"
	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	preparationdomain "issueops/internal/domain/issueopspreparation"
	"issueops/internal/port"
)

type reconcileEffectsFake struct {
	state ReconcileEffectState
}

func (f *reconcileEffectsFake) ApplyReceipt(context.Context, ReconcileEffectState, leasecontract.ReconcileStageReceipt) (ReconcileEffectState, error) {
	return f.state, nil
}

func TestReconcileRepositoryPreservesRawCASState(t *testing.T) {
	_, sealed, store := seededResumeIntent(t)
	state, err := NewReconcileRepository(store, nil).Canonicalize(context.Background(), sealed.Progress.Record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(state.RecordRaw, sealed.RecordRaw) || !bytes.Equal(state.IntentRaw, sealed.IntentRaw) || state.OperationID != sealed.OperationID {
		t.Fatalf("state = %#v", state)
	}
}

func TestReconcileRepositoryRejectsStaleSnapshotBeforeReadingIntent(t *testing.T) {
	_, sealed, store := seededResumeIntent(t)
	repository := NewReconcileRepository(store, nil)
	snapshot := sealed.Progress.Record.Stable
	snapshot.Execution = &leasecontract.Execution{}
	repository.snapshot = &snapshot
	_, err := repository.Canonicalize(context.Background(), sealed.Progress.Record.ID)
	if err == nil || !strings.Contains(err.Error(), "orca_intent_authority_changed") {
		t.Fatalf("stale snapshot error=%v", err)
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

func TestReconcileRepositoryAppliesResumeReceiptWithoutBridge(t *testing.T) {
	_, sealed, store := seededResumeIntent(t)
	repository := NewReconcileRepository(store, nil)
	intent := leaseapp.ReconcileIntentState{
		Progress:    leaseapp.ReconcileProgress{Record: sealed.Progress.Record.Stable, Pending: true, NextStage: sealed.Stage},
		OperationID: sealed.OperationID, Stage: sealed.Stage, InvocationState: sealed.InvocationState,
		RecordRaw: sealed.RecordRaw, IntentRaw: sealed.IntentRaw,
	}
	progress, err := repository.ApplyReceipt(context.Background(), intent, leasecontract.ReconcileStageReceipt{TerminalPTYID: "pty-recovered"})
	if err != nil {
		t.Fatal(err)
	}
	if !progress.Pending || progress.NextStage != "run_create" || progress.Record.Execution.Pending.Kind != "owner_launch" {
		t.Fatalf("reconcile progress=%+v", progress)
	}
	if _, err := repository.ApplyReceipt(context.Background(), intent, leasecontract.ReconcileStageReceipt{TerminalPTYID: "pty-again"}); err == nil {
		t.Fatal("stale reconcile receipt was accepted")
	}
}

func TestReconcileRepositoryAdvancesPreparedOwnerReceiptWithoutBridge(t *testing.T) {
	_, sealed, store := seededResumeIntent(t)
	record := sealed.Progress.Record.Stable
	record.Execution.Lease.Generation = 1
	record.Execution.Lease.Status = "released"
	record.Execution.Lease.ClaimTokenSHA256 = ""
	record.Execution.Orca = nil
	payload, err := (preparationcontract.IntentCodec{}).Decode(sealed.OperationID, sealed.IntentRaw)
	if err != nil {
		t.Fatal(err)
	}
	payload.Purpose = preparationcontract.PurposePrepare
	payload.Generation = 1
	payload.PriorBinding, payload.ResumeLease = nil, nil
	payload, err = preparationdomain.SealIntent(payload, preparationcontract.IssueIdentity{Provider: "github", Issue: 193})
	if err != nil {
		t.Fatal(err)
	}
	record.Execution.Pending.Marker = payload.Marker
	recordRaw, err := leasecontract.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	intentRaw, err := (preparationcontract.IntentCodec{}).Encode(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Apply(context.Background(), []port.RecordMutation{
		{Bucket: recordBucket, ID: record.ID, Data: recordRaw},
		{Bucket: "external_intent_v1", ID: payload.OperationID, Data: intentRaw},
	}); err != nil {
		t.Fatal(err)
	}
	repository := NewReconcileRepository(store, nil)
	state := leaseapp.ReconcileIntentState{
		Progress:    leaseapp.ReconcileProgress{Record: record, Pending: true, NextStage: string(payload.Stage)},
		OperationID: payload.OperationID, Stage: string(payload.Stage), InvocationState: payload.InvocationState,
		RecordRaw: recordRaw, IntentRaw: intentRaw,
	}
	progress, err := repository.ApplyReceipt(context.Background(), state, leasecontract.ReconcileStageReceipt{TerminalPTYID: "pty-prepared"})
	if err != nil {
		t.Fatal(err)
	}
	if !progress.Pending || progress.NextStage != string(preparationcontract.IntentStageRun) || progress.Record.Execution.Pending.Kind != "owner_launch" || progress.Record.Execution.Lease.Status != "released" {
		t.Fatalf("prepare receipt progress=%+v", progress)
	}
	if _, err := repository.ApplyReceipt(context.Background(), state, leasecontract.ReconcileStageReceipt{TerminalPTYID: "pty-again"}); err == nil {
		t.Fatal("stale prepare receipt was accepted")
	}
	for _, step := range []struct {
		receipt   leasecontract.ReconcileStageReceipt
		nextStage preparationcontract.IntentStage
	}{
		{leasecontract.ReconcileStageReceipt{RunID: "run-prepared"}, preparationcontract.IntentStageRunBind},
		{leasecontract.ReconcileStageReceipt{RunID: "run-prepared", RunBound: true}, preparationcontract.IntentStageTask},
		{leasecontract.ReconcileStageReceipt{TaskID: "task-prepared"}, preparationcontract.IntentStageDispatch},
	} {
		state, err = repository.Canonicalize(context.Background(), record.ID)
		if err != nil {
			t.Fatal(err)
		}
		progress, err = repository.ApplyReceipt(context.Background(), state, step.receipt)
		if err != nil {
			t.Fatal(err)
		}
		if !progress.Pending || progress.NextStage != string(step.nextStage) {
			t.Fatalf("prepare stage progress=%+v", progress)
		}
	}
	state, err = repository.Canonicalize(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	progress, err = repository.ApplyReceipt(context.Background(), state, leasecontract.ReconcileStageReceipt{
		TerminalPTYID: "pty-prepared", TerminalHandle: "handle-prepared", TaskID: "task-prepared",
		DispatchID: "dispatch-prepared", RequestID: "11111111-1111-4111-8111-111111111111",
	})
	if err != nil {
		t.Fatal(err)
	}
	if progress.Pending || progress.Record.Execution.Pending != nil || progress.Record.Execution.Lease.Status != "claimable" || progress.Record.Execution.Orca == nil || progress.Record.Execution.Orca.DispatchID != "dispatch-prepared" {
		t.Fatalf("completed prepare progress=%+v", progress)
	}
	if _, ok, err := store.Get("external_intent_v1", payload.OperationID); err != nil || ok {
		t.Fatalf("completed prepare intent remains: present=%v err=%v", ok, err)
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

func TestReconcileRepositoryClearIntentDeletesOnlySealedPendingIntent(t *testing.T) {
	_, state, store := seededResumeIntent(t)
	repository := NewReconcileRepository(store, nil)
	repository.now = func() time.Time { return time.Date(2026, time.July, 31, 3, 10, 0, 0, time.UTC) }
	repository.redact = func(string) string { return "no resource observed" }
	intent := leaseapp.ReconcileIntentState{
		Progress:    leaseapp.ReconcileProgress{Record: state.Progress.Record.Stable, Pending: true, NextStage: state.Stage},
		OperationID: state.OperationID, Stage: state.Stage, InvocationState: state.InvocationState,
		InvocationAttempts: state.InvocationAttempts, RecordRaw: state.RecordRaw, IntentRaw: state.IntentRaw,
	}
	progress, err := repository.ClearIntent(context.Background(), intent, errors.New("sensitive detail"))
	if err != nil {
		t.Fatal(err)
	}
	if progress.Pending || progress.Record.Execution.Pending != nil {
		t.Fatalf("progress=%+v", progress)
	}
	if got := progress.Record.Execution.Failure; got == nil || got.Code != "external_operation_left_no_resource" || got.Message != "no resource observed" || got.At != "2026-07-31T03:10:00Z" {
		t.Fatalf("failure=%+v", got)
	}
	if _, ok, err := store.Get("external_intent_v1", intent.OperationID); err != nil || ok {
		t.Fatalf("intent still present=%v err=%v", ok, err)
	}
	if _, err := repository.ClearIntent(context.Background(), intent, errors.New("retry")); err == nil {
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
