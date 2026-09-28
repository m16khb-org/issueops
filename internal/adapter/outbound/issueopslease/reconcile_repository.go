package issueopslease

import (
	"context"
	"fmt"

	leaseapp "issueops/internal/application/issueopslease"
	leasecontract "issueops/internal/contract/issueopslease"
	"issueops/internal/port"
)

type ReconcileEffects interface {
	Canonicalize(context.Context, string) (ReconcileEffectState, error)
	RecordFailure(context.Context, ReconcileEffectState, string, error) error
	ApplyReceipt(context.Context, ReconcileEffectState, leasecontract.ReconcileStageReceipt) (ReconcileEffectState, error)
	// ClearIntent는 외부 자원이 없음이 authoritative하게 확인된 intent를
	// 제거한다. 재시도가 아니라 기록 정리다(#280).
	ClearIntent(context.Context, ReconcileEffectState, error) (ReconcileEffectState, error)
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
}

func NewReconcileRepository(store port.TransactionalRecordStore, effects ReconcileEffects) *ReconcileRepository {
	return &ReconcileRepository{store: store, effects: effects}
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
	if r == nil || r.effects == nil {
		return fmt.Errorf("reconcile persistence bridge is required")
	}
	return r.effects.RecordFailure(ctx, reconcileEffectState(intent), invocation, cause)
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
	if r == nil || r.effects == nil {
		return leaseapp.ReconcileProgress{}, fmt.Errorf("reconcile persistence bridge is required")
	}
	next, err := r.effects.ClearIntent(ctx, reconcileEffectState(state), cause)
	if err != nil {
		return leaseapp.ReconcileProgress{}, err
	}
	return leaseapp.ReconcileProgress{Record: next.Record, Pending: false}, nil
}
