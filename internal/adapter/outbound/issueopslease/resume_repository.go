package issueopslease

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	recordcodec "issueops/internal/adapter/outbound/issueopsrecord"
	leaseapp "issueops/internal/application/issueopslease"
	preparationapp "issueops/internal/application/issueopspreparation"
	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	leasedomain "issueops/internal/domain/issueopslease"
	preparationdomain "issueops/internal/domain/issueopspreparation"
	"issueops/internal/port"
)

type ResumeEffects interface {
	Begin(context.Context, leasecontract.Record, []byte, leasecontract.ResumeArtifacts, leasedomain.ResumePlan, string) (ResumeEffectState, error)
	Read(context.Context, string, string) (ResumeEffectState, error)
	ApplyReceipt(context.Context, ResumeEffectState, leasecontract.ResumeStageReceipt) (ResumeEffectState, error)
}

type ResumeEffectState struct {
	Record             leasecontract.Record
	RecordRaw          []byte
	IntentRaw          []byte
	OperationID        string
	Stage              string
	InvocationState    string
	InvocationAttempts int
	Pending            bool
}

type ResumeRepository struct {
	store   port.TransactionalRecordStore
	effects ResumeEffects
	now     func() time.Time
	redact  func(string) string
}

func NewResumeRepository(store port.TransactionalRecordStore, effects ResumeEffects) *ResumeRepository {
	return NewResumeRepositoryWithDiagnosticRedactor(store, effects, nil, time.Now)
}

func NewResumeRepositoryWithDiagnosticRedactor(store port.TransactionalRecordStore, effects ResumeEffects, redact func(string) string, now func() time.Time) *ResumeRepository {
	return &ResumeRepository{store: store, effects: effects, redact: redact, now: now}
}

func (r *ResumeRepository) LoadSnapshot(_ context.Context, id string, generation uint64) (leaseapp.ResumeSnapshot, error) {
	if r == nil || r.store == nil {
		return leaseapp.ResumeSnapshot{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("transactional record store is required"))
	}
	data, ok, err := r.store.Get(recordBucket, id)
	if err != nil {
		return leaseapp.ResumeSnapshot{}, leasecontract.Fail(leasecontract.FailurePersistence, err)
	}
	if !ok {
		return leaseapp.ResumeSnapshot{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("issueops record %s not found", id))
	}
	record, err := decodeLeaseRecord(id, data)
	if err != nil {
		return leaseapp.ResumeSnapshot{}, err
	}
	if record.Execution == nil {
		return leaseapp.ResumeSnapshot{}, leasecontract.Fail(leasecontract.FailurePersistence, leasecontract.ErrExecutionNotPrepared)
	}
	if generation == 0 || record.Execution.Lease.Generation != generation {
		return leaseapp.ResumeSnapshot{}, fmt.Errorf("stale lease generation: current=%d expected=%d", record.Execution.Lease.Generation, generation)
	}
	return leaseapp.ResumeSnapshot{Record: toApplicationRecord(record), Raw: append([]byte(nil), data...)}, nil
}

func (r *ResumeRepository) BeginIntent(ctx context.Context, snapshot leaseapp.ResumeSnapshot, artifacts leasecontract.ResumeArtifacts, plan leasedomain.ResumePlan, operationID string) (leaseapp.ResumeProgress, error) {
	if r == nil || r.effects == nil {
		return leaseapp.ResumeProgress{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("resume persistence bridge is required"))
	}
	state, err := r.effects.Begin(ctx, snapshot.Record.Stable, snapshot.Raw, artifacts, plan, operationID)
	if err != nil {
		return leaseapp.ResumeProgress{}, err
	}
	return resumeProgress(state), nil
}

func (r *ResumeRepository) LoadIntent(ctx context.Context, progress leaseapp.ResumeProgress) (leaseapp.ResumeIntentState, error) {
	if r == nil || r.effects == nil || progress.Execution.Pending == nil {
		return leaseapp.ResumeIntentState{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("resume pending intent is required"))
	}
	state, err := r.effects.Read(ctx, progress.Record.ID, progress.Execution.Pending.OperationID)
	if err != nil {
		return leaseapp.ResumeIntentState{}, err
	}
	return resumeIntentState(state), nil
}

func (r *ResumeRepository) MarkInvoking(ctx context.Context, intent leaseapp.ResumeIntentState) (leaseapp.ResumeIntentState, error) {
	if r == nil || r.store == nil {
		return leaseapp.ResumeIntentState{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("transactional record store is required"))
	}
	store, ok := r.store.(port.RecordRawCASStore)
	if !ok {
		return leaseapp.ResumeIntentState{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("resume record store does not support raw CAS"))
	}
	if len(intent.RecordRaw) == 0 || len(intent.IntentRaw) == 0 {
		return leaseapp.ResumeIntentState{}, fmt.Errorf("Orca intent raw CAS evidence is required")
	}
	codec := preparationcontract.IntentCodec{}
	current, err := codec.Decode(intent.OperationID, intent.IntentRaw)
	if err != nil {
		return leaseapp.ResumeIntentState{}, err
	}
	if err := preparationdomain.ValidateIntentRecord(intent.Progress.Record.Stable, current); err != nil {
		return leaseapp.ResumeIntentState{}, err
	}
	updated := preparationapp.MarkOrcaInvoking(current)
	data, err := codec.Encode(updated)
	if err != nil {
		return leaseapp.ResumeIntentState{}, err
	}
	err = store.CompareAndApply(ctx, []port.ExpectedRecord{
		{Bucket: recordBucket, ID: intent.Progress.Record.ID, Data: intent.RecordRaw},
		{Bucket: "external_intent_v1", ID: intent.OperationID, Data: intent.IntentRaw},
	}, []port.RecordMutation{{Bucket: "external_intent_v1", ID: intent.OperationID, Data: data}})
	if err != nil {
		if stale, ok := errors.AsType[port.RawCASFailure](err); ok {
			if stale.FailedBucket() == recordBucket {
				return leaseapp.ResumeIntentState{}, fmt.Errorf("stale raw record snapshot")
			}
			return leaseapp.ResumeIntentState{}, fmt.Errorf("stale raw intent snapshot")
		}
		return leaseapp.ResumeIntentState{}, err
	}
	intent.IntentRaw = data
	intent.InvocationState = updated.InvocationState
	intent.InvocationAttempts = updated.InvocationAttempts
	return intent, nil
}

func (r *ResumeRepository) RecordFailure(ctx context.Context, intent leaseapp.ResumeIntentState, invocation string, cause error) error {
	if r == nil || r.store == nil {
		return leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("transactional record store is required"))
	}
	store, ok := r.store.(port.RecordRawCASStore)
	if !ok {
		return leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("resume record store does not support raw CAS"))
	}
	if len(intent.RecordRaw) == 0 || len(intent.IntentRaw) == 0 {
		return fmt.Errorf("Orca intent raw CAS evidence is required")
	}
	codec := preparationcontract.IntentCodec{}
	current, err := codec.Decode(intent.OperationID, intent.IntentRaw)
	if err != nil {
		return err
	}
	state := preparationapp.IntentState{Snapshot: preparationcontract.Snapshot{Record: intent.Progress.Record.Stable, RecordRaw: intent.RecordRaw}, Intent: current, IntentRaw: intent.IntentRaw, FailureAt: r.now().UTC().Format(time.RFC3339Nano)}
	if err := preparationapp.ValidateIntentState(state); err != nil {
		return err
	}
	record, updated, err := preparationapp.ApplyOrcaFailure(state, invocation, func() string { return r.boundedDiagnostic(cause) })
	if err != nil {
		return err
	}
	if typed, ok := errors.AsType[*port.OrcaError](cause); ok {
		updated.OrcaRequestID, updated.OrcaPromptRequestID = preparationdomain.AdoptFailureRequestIDs(preparationdomain.FailureRequestIDFacts{
			SealedDispatch: updated.OrcaRequestID, SealedPrompt: updated.OrcaPromptRequestID,
			CallPhase: typed.CallPhase, ObservedDispatch: typed.DispatchRequestID,
			ObservedOrchestration: typed.OrchestrationRequestID,
			DispatchValid:         port.ValidateOrcaRequestID(typed.DispatchRequestID) == nil,
			OrchestrationValid:    port.ValidateOrcaRequestID(typed.OrchestrationRequestID) == nil,
		})
	}
	recordData, err := recordcodec.EncodeLease(record)
	if err != nil {
		return err
	}
	intentData, err := codec.Encode(updated)
	if err != nil {
		return err
	}
	err = store.CompareAndApply(ctx, []port.ExpectedRecord{
		{Bucket: recordBucket, ID: record.ID, Data: intent.RecordRaw},
		{Bucket: "external_intent_v1", ID: intent.OperationID, Data: intent.IntentRaw},
	}, []port.RecordMutation{
		{Bucket: recordBucket, ID: record.ID, Data: recordData},
		{Bucket: "external_intent_v1", ID: intent.OperationID, Data: intentData},
	})
	if stale, ok := errors.AsType[port.RawCASFailure](err); ok {
		if stale.FailedBucket() == recordBucket {
			return fmt.Errorf("stale raw record snapshot")
		}
		return fmt.Errorf("stale raw intent snapshot")
	}
	return err
}

func (r *ResumeRepository) boundedDiagnostic(cause error) string {
	message := "external operation failed"
	if r.redact != nil && cause != nil {
		message = strings.TrimSpace(r.redact(cause.Error()))
		if message == "" {
			message = "external operation failed"
		}
	}
	if len(message) > 4096 {
		message = message[:4096]
	}
	return message
}

func (r *ResumeRepository) ApplyReceipt(ctx context.Context, intent leaseapp.ResumeIntentState, receipt leasecontract.ResumeStageReceipt) (leaseapp.ResumeProgress, error) {
	if r == nil || r.effects == nil {
		return leaseapp.ResumeProgress{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("resume persistence bridge is required"))
	}
	state, err := r.effects.ApplyReceipt(ctx, resumeEffectState(intent), receipt)
	if err != nil {
		return leaseapp.ResumeProgress{}, err
	}
	return resumeProgress(state), nil
}

func resumeProgress(state ResumeEffectState) leaseapp.ResumeProgress {
	return leaseapp.ResumeProgress{Record: toApplicationRecord(state.Record), Execution: *state.Record.Execution, Pending: state.Pending}
}

func resumeIntentState(state ResumeEffectState) leaseapp.ResumeIntentState {
	return leaseapp.ResumeIntentState{Progress: resumeProgress(state), OperationID: state.OperationID, Stage: state.Stage, InvocationState: state.InvocationState, InvocationAttempts: state.InvocationAttempts, RecordRaw: append([]byte(nil), state.RecordRaw...), IntentRaw: append([]byte(nil), state.IntentRaw...)}
}

func resumeEffectState(intent leaseapp.ResumeIntentState) ResumeEffectState {
	return ResumeEffectState{Record: intent.Progress.Record.Stable, RecordRaw: append([]byte(nil), intent.RecordRaw...), IntentRaw: append([]byte(nil), intent.IntentRaw...), OperationID: intent.OperationID, Stage: intent.Stage, InvocationState: intent.InvocationState, InvocationAttempts: intent.InvocationAttempts, Pending: intent.Progress.Pending}
}
