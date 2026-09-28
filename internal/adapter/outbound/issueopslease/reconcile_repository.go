package issueopslease

import (
	"context"
	"errors"
	"fmt"
	"time"

	recordcodec "issueops/internal/adapter/outbound/issueopsrecord"
	leaseapp "issueops/internal/application/issueopslease"
	preparationapp "issueops/internal/application/issueopspreparation"
	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	preparationdomain "issueops/internal/domain/issueopspreparation"
	"issueops/internal/port"
)

type ReconcileEffects interface {
	ApplyReceipt(context.Context, ReconcileEffectState, leasecontract.ReconcileStageReceipt) (ReconcileEffectState, error)
}

type ReconcileEffectState struct {
	Record             leasecontract.Record
	RecordRaw          []byte
	IntentRaw          []byte
	OperationID        string
	Stage              string
	InvocationState    string
	InvocationAttempts int
	Pending            bool
}

type ReconcileRepository struct {
	store    port.TransactionalRecordStore
	effects  ReconcileEffects
	now      func() time.Time
	redact   func(string) string
	snapshot *leasecontract.Record
}

func NewReconcileRepository(store port.TransactionalRecordStore, effects ReconcileEffects) *ReconcileRepository {
	return NewReconcileRepositoryWithDiagnosticRedactor(store, effects, nil, time.Now)
}

func NewReconcileRepositoryWithDiagnosticRedactor(store port.TransactionalRecordStore, effects ReconcileEffects, redact func(string) string, now func() time.Time) *ReconcileRepository {
	return NewReconcileRepositoryWithSnapshot(store, effects, nil, redact, now)
}

func NewReconcileRepositoryWithSnapshot(store port.TransactionalRecordStore, effects ReconcileEffects, snapshot *leasecontract.Record, redact func(string) string, now func() time.Time) *ReconcileRepository {
	if now == nil {
		now = time.Now
	}
	return &ReconcileRepository{store: store, effects: effects, snapshot: snapshot, redact: redact, now: now}
}

func (r *ReconcileRepository) Canonicalize(ctx context.Context, id string) (leaseapp.ReconcileIntentState, error) {
	result := leaseapp.ReconcileIntentState{Progress: leaseapp.ReconcileProgress{Record: leasecontract.Record{ID: id}}}
	if r == nil || r.store == nil {
		return result, fmt.Errorf("reconcile record store is required")
	}
	err := r.store.WithSpan(ctx, func(context.Context) error {
		recordRaw, ok, err := r.store.Get(recordBucket, id)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("issueops record %s not found", id)
		}
		record, err := decodeLeaseRecord(id, recordRaw)
		if err != nil {
			return err
		}
		result.Progress.Record = record
		expected := record
		if r.snapshot != nil {
			expected = *r.snapshot
		}
		if err := preparationdomain.ValidateReconcileSnapshot(record, expected); err != nil {
			return err
		}
		operationID := record.Execution.Pending.OperationID
		intentRaw, ok, err := r.store.Get("external_intent_v1", operationID)
		if err != nil {
			return err
		}
		if !ok {
			return &preparationcontract.IntentError{Code: "orca_intent_invalid", Detail: "Orca external intent payload is missing"}
		}
		intent, _, err := preparationdomain.CanonicalizeIntent(record, intentRaw)
		if err != nil {
			return err
		}
		if err := preparationdomain.ValidateReconcileIntentIssueIdentity(record, intent); err != nil {
			return err
		}
		result = leaseapp.ReconcileIntentState{
			Progress:    leaseapp.ReconcileProgress{Record: record, Pending: true, NextStage: string(intent.Stage)},
			OperationID: operationID, Stage: string(intent.Stage), InvocationState: intent.InvocationState,
			InvocationAttempts: intent.InvocationAttempts,
			RecordRaw:          append([]byte(nil), recordRaw...), IntentRaw: append([]byte(nil), intentRaw...),
		}
		return nil
	})
	return result, err
}

func (r *ReconcileRepository) MarkInvoking(ctx context.Context, intent leaseapp.ReconcileIntentState) (leaseapp.ReconcileIntentState, error) {
	if r == nil {
		return leaseapp.ReconcileIntentState{}, fmt.Errorf("reconcile record store is required")
	}
	updated, data, err := markOrcaIntentInvoking(ctx, r.store, intent.Progress.Record, intent.OperationID, intent.RecordRaw, intent.IntentRaw)
	if err != nil {
		return leaseapp.ReconcileIntentState{}, err
	}
	intent.IntentRaw = data
	intent.InvocationState = updated.InvocationState
	intent.InvocationAttempts = updated.InvocationAttempts
	return intent, nil
}

func (r *ReconcileRepository) RecordFailure(ctx context.Context, intent leaseapp.ReconcileIntentState, invocation string, cause error) error {
	if r == nil || r.store == nil {
		return fmt.Errorf("reconcile record store is required")
	}
	store, ok := r.store.(port.RecordRawCASStore)
	if !ok {
		return fmt.Errorf("reconcile record store does not support raw CAS")
	}
	return recordOrcaIntentFailure(ctx, store, orcaFailureState{
		Record: intent.Progress.Record, RecordRaw: intent.RecordRaw,
		IntentRaw: intent.IntentRaw, OperationID: intent.OperationID,
	}, invocation, cause, r.now, r.redact)
}

func (r *ReconcileRepository) ApplyReceipt(ctx context.Context, intent leaseapp.ReconcileIntentState, receipt leasecontract.ReconcileStageReceipt) (leaseapp.ReconcileProgress, error) {
	if r == nil || r.store == nil {
		return leaseapp.ReconcileProgress{}, fmt.Errorf("reconcile record store is required")
	}
	payload, err := (preparationcontract.IntentCodec{}).Decode(intent.OperationID, intent.IntentRaw)
	if err != nil {
		return leaseapp.ReconcileProgress{}, err
	}
	if payload.Purpose == preparationcontract.PurposeResume {
		if intent.Progress.Record.Execution == nil {
			return leaseapp.ReconcileProgress{}, fmt.Errorf("reconcile execution is required")
		}
		nextStage, complete, err := preparationdomain.NextOrcaReceiptStage(payload.Stage)
		if err != nil {
			return leaseapp.ReconcileProgress{}, err
		}
		resume := leaseapp.ResumeIntentState{
			Progress: leaseapp.ResumeProgress{
				Record: toApplicationRecord(intent.Progress.Record), Execution: *intent.Progress.Record.Execution,
				Pending: intent.Progress.Pending,
			},
			OperationID: intent.OperationID, Stage: intent.Stage, InvocationState: intent.InvocationState,
			InvocationAttempts: intent.InvocationAttempts, RecordRaw: intent.RecordRaw, IntentRaw: intent.IntentRaw,
		}
		progress, err := NewResumeRepository(r.store).ApplyReceipt(ctx, resume, leasecontract.ResumeStageReceipt{
			TerminalPTYID: receipt.TerminalPTYID, TerminalHandle: receipt.TerminalHandle,
			RunID: receipt.RunID, RunBound: receipt.RunBound, TaskID: receipt.TaskID,
			DispatchID: receipt.DispatchID, RequestID: receipt.RequestID, PromptReceipt: receipt.PromptReceipt,
		})
		if err != nil {
			return leaseapp.ReconcileProgress{}, err
		}
		result := leaseapp.ReconcileProgress{Record: progress.Record.Stable, Pending: progress.Pending}
		if !complete {
			result.NextStage = string(nextStage)
		}
		return result, nil
	}
	if r.effects == nil {
		return leaseapp.ReconcileProgress{}, fmt.Errorf("reconcile persistence bridge is required")
	}
	state, err := r.effects.ApplyReceipt(ctx, reconcileEffectState(intent), receipt)
	return reconcileProgress(state), err
}

func (r *ReconcileRepository) Latest(_ context.Context, id string) (leasecontract.Record, error) {
	if r == nil || r.store == nil {
		return leasecontract.Record{}, fmt.Errorf("reconcile record store is required")
	}
	data, ok, err := r.store.Get(recordBucket, id)
	if err != nil {
		return leasecontract.Record{}, err
	}
	if !ok {
		return leasecontract.Record{}, fmt.Errorf("issueops record %s not found", id)
	}
	return decodeLeaseRecord(id, data)
}

func reconcileProgress(state ReconcileEffectState) leaseapp.ReconcileProgress {
	return leaseapp.ReconcileProgress{Record: state.Record, Pending: state.Pending, NextStage: state.Stage}
}

func reconcileEffectState(intent leaseapp.ReconcileIntentState) ReconcileEffectState {
	return ReconcileEffectState{
		Record: intent.Progress.Record, RecordRaw: append([]byte(nil), intent.RecordRaw...), IntentRaw: append([]byte(nil), intent.IntentRaw...),
		OperationID: intent.OperationID, Stage: intent.Stage, InvocationState: intent.InvocationState,
		InvocationAttempts: intent.InvocationAttempts, Pending: intent.Progress.Pending,
	}
}

// ClearIntent는 authoritative zero로 확인된 intent를 제거하고 진행 상태를
// 돌려준다. stage를 전진시키지 않으므로 Pending은 false다.
func (r *ReconcileRepository) ClearIntent(ctx context.Context, state leaseapp.ReconcileIntentState, cause error) (leaseapp.ReconcileProgress, error) {
	if r == nil || r.store == nil {
		return leaseapp.ReconcileProgress{}, fmt.Errorf("reconcile record store is required")
	}
	store, ok := r.store.(port.RecordRawCASStore)
	if !ok {
		return leaseapp.ReconcileProgress{}, fmt.Errorf("reconcile record store does not support raw CAS")
	}
	if len(state.RecordRaw) == 0 || len(state.IntentRaw) == 0 {
		return leaseapp.ReconcileProgress{}, fmt.Errorf("Orca intent raw CAS evidence is required")
	}
	intent, err := (preparationcontract.IntentCodec{}).Decode(state.OperationID, state.IntentRaw)
	if err != nil {
		return leaseapp.ReconcileProgress{}, err
	}
	current := preparationapp.IntentState{Snapshot: preparationcontract.Snapshot{Record: state.Progress.Record, RecordRaw: state.RecordRaw}, Intent: intent, IntentRaw: state.IntentRaw, FailureAt: r.now().UTC().Format(time.RFC3339Nano)}
	if err := preparationapp.ValidateIntentState(current); err != nil {
		return leaseapp.ReconcileProgress{}, err
	}
	record, err := preparationapp.ApplyOrcaNoResource(current, func() string { return boundedOrcaFailureDiagnostic(cause, r.redact) })
	if err != nil {
		return leaseapp.ReconcileProgress{}, err
	}
	data, err := recordcodec.EncodeLease(record)
	if err != nil {
		return leaseapp.ReconcileProgress{}, err
	}
	err = store.CompareAndApply(ctx, []port.ExpectedRecord{
		{Bucket: recordBucket, ID: record.ID, Data: state.RecordRaw},
		{Bucket: "external_intent_v1", ID: state.OperationID, Data: state.IntentRaw},
	}, []port.RecordMutation{
		{Bucket: recordBucket, ID: record.ID, Data: data},
		{Bucket: "external_intent_v1", ID: state.OperationID, Delete: true},
	})
	if stale, ok := errors.AsType[port.RawCASFailure](err); ok {
		if stale.FailedBucket() == recordBucket {
			return leaseapp.ReconcileProgress{}, fmt.Errorf("stale raw record snapshot")
		}
		return leaseapp.ReconcileProgress{}, fmt.Errorf("stale raw intent snapshot")
	}
	if err != nil {
		return leaseapp.ReconcileProgress{}, err
	}
	return leaseapp.ReconcileProgress{Record: record, Pending: false}, nil
}
