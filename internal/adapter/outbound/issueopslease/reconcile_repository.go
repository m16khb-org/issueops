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
	"issueops/internal/port"
)

type ReconcileEffects interface {
	Canonicalize(context.Context, string) (ReconcileEffectState, error)
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
	store   port.TransactionalRecordStore
	effects ReconcileEffects
	now     func() time.Time
	redact  func(string) string
}

func NewReconcileRepository(store port.TransactionalRecordStore, effects ReconcileEffects) *ReconcileRepository {
	return NewReconcileRepositoryWithDiagnosticRedactor(store, effects, nil, time.Now)
}

func NewReconcileRepositoryWithDiagnosticRedactor(store port.TransactionalRecordStore, effects ReconcileEffects, redact func(string) string, now func() time.Time) *ReconcileRepository {
	if now == nil {
		now = time.Now
	}
	return &ReconcileRepository{store: store, effects: effects, redact: redact, now: now}
}

func (r *ReconcileRepository) Canonicalize(ctx context.Context, id string) (leaseapp.ReconcileIntentState, error) {
	if r == nil || r.effects == nil {
		return leaseapp.ReconcileIntentState{}, fmt.Errorf("reconcile persistence bridge is required")
	}
	state, err := r.effects.Canonicalize(ctx, id)
	return reconcileIntentState(state), err
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
	if r == nil || r.effects == nil {
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

func reconcileIntentState(state ReconcileEffectState) leaseapp.ReconcileIntentState {
	return leaseapp.ReconcileIntentState{
		Progress: reconcileProgress(state), OperationID: state.OperationID, Stage: state.Stage,
		InvocationState: state.InvocationState, InvocationAttempts: state.InvocationAttempts,
		RecordRaw: append([]byte(nil), state.RecordRaw...), IntentRaw: append([]byte(nil), state.IntentRaw...),
	}
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
