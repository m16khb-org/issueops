package issueopslease

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	recordcodec "issueops/internal/adapter/outbound/issueopsrecord"
	leaseapp "issueops/internal/application/issueopslease"
	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	leasedomain "issueops/internal/domain/issueopslease"
	preparationdomain "issueops/internal/domain/issueopspreparation"
	"issueops/internal/port"
)

type ResumeRepository struct {
	store  port.TransactionalRecordStore
	now    func() time.Time
	redact func(string) string
}

func NewResumeRepository(store port.TransactionalRecordStore) *ResumeRepository {
	return NewResumeRepositoryWithDiagnosticRedactor(store, nil, time.Now)
}

func NewResumeRepositoryWithDiagnosticRedactor(store port.TransactionalRecordStore, redact func(string) string, now func() time.Time) *ResumeRepository {
	if now == nil {
		now = time.Now
	}
	return &ResumeRepository{store: store, redact: redact, now: now}
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
	record, err := decodeMutableLeaseRecord(id, data)
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
	if err := recordcodec.RequireMutableLeaseSnapshot(snapshot.Record.Stable, snapshot.Raw); err != nil {
		return leaseapp.ResumeProgress{}, err
	}
	if r == nil || r.store == nil {
		return leaseapp.ResumeProgress{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("transactional record store is required"))
	}
	store, ok := r.store.(port.RecordRawCASStore)
	if !ok {
		return leaseapp.ResumeProgress{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("resume record store does not support raw CAS"))
	}
	if len(snapshot.Raw) == 0 {
		return leaseapp.ResumeProgress{}, fmt.Errorf("resume raw record snapshot is required")
	}
	record := snapshot.Record.Stable
	if record.Execution == nil || record.Execution.Orca == nil {
		return leaseapp.ResumeProgress{}, fmt.Errorf("execution resume requires an existing Orca binding")
	}
	workspace, err := preparationdomain.ResumeWorkspace(record)
	if err != nil {
		return leaseapp.ResumeProgress{}, err
	}
	matcher := FilesystemPathMatcher{}
	if workspace.ExpectedParent != "" && !matcher.Matches(workspace.Request.ParentWorktree, workspace.ExpectedParent) {
		return leaseapp.ResumeProgress{}, fmt.Errorf("parent_worktree %q does not match canonical parent worktree %q", workspace.Request.ParentWorktree, workspace.ExpectedParent)
	}
	if !matcher.Matches(workspace.Request.Root, record.Execution.Workspace.Root) {
		return leaseapp.ResumeProgress{}, fmt.Errorf("execution resume workspace is not canonical")
	}
	intent, err := preparationdomain.BuildResumeIntent(preparationdomain.ResumeIntentInput{
		Record: record, Workspace: workspace.Request, Artifacts: artifacts,
		RuntimeID: plan.RuntimeID, ReusedTerminalPTYID: plan.ReusedTerminalPTYID,
		OperationID: operationID, StartedAt: r.now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return leaseapp.ResumeProgress{}, err
	}
	currentRaw, ok, err := r.store.Get(recordBucket, record.ID)
	if err != nil {
		return leaseapp.ResumeProgress{}, err
	}
	if !ok {
		return leaseapp.ResumeProgress{}, fmt.Errorf("issueops record %s not found", record.ID)
	}
	current, err := decodeMutableLeaseRecord(record.ID, currentRaw)
	if err != nil {
		return leaseapp.ResumeProgress{}, err
	}
	if current.Execution == nil {
		return leaseapp.ResumeProgress{}, leasecontract.Fail(leasecontract.FailurePersistence, leasecontract.ErrExecutionNotPrepared)
	}
	if current.Execution.Lease.Generation != record.Execution.Lease.Generation {
		return leaseapp.ResumeProgress{}, fmt.Errorf("stale lease generation: current=%d expected=%d", current.Execution.Lease.Generation, record.Execution.Lease.Generation)
	}
	if err := leasedomain.ValidateResumeBeginAuthority(leasedomain.ResumeBeginAuthority{
		Pending:     current.Execution.Pending != nil,
		LeaseSame:   reflect.DeepEqual(current.Execution.Lease, record.Execution.Lease),
		BindingSame: reflect.DeepEqual(current.Execution.Orca, record.Execution.Orca),
	}); err != nil {
		return leaseapp.ResumeProgress{}, err
	}
	if err := preparationdomain.ValidateReconcileIntentIssueIdentity(current, intent); err != nil {
		return leaseapp.ResumeProgress{}, err
	}
	next := preparationdomain.ApplyResumeIntent(current, intent)
	recordData, err := recordcodec.EncodeLease(next)
	if err != nil {
		return leaseapp.ResumeProgress{}, err
	}
	intentData, err := (preparationcontract.IntentCodec{}).Encode(intent)
	if err != nil {
		return leaseapp.ResumeProgress{}, err
	}
	err = store.CompareAndApply(ctx, []port.ExpectedRecord{{Bucket: recordBucket, ID: record.ID, Data: snapshot.Raw}}, []port.RecordMutation{
		{Bucket: recordBucket, ID: record.ID, Data: recordData},
		{Bucket: "external_intent_v1", ID: operationID, Data: intentData, RequireAbsent: true},
	})
	if stale, ok := errors.AsType[port.RawCASFailure](err); ok {
		if stale.FailedBucket() == recordBucket {
			return leaseapp.ResumeProgress{}, fmt.Errorf("stale raw record snapshot")
		}
		return leaseapp.ResumeProgress{}, fmt.Errorf("stale raw intent snapshot")
	}
	if err != nil {
		return leaseapp.ResumeProgress{}, err
	}
	return leaseapp.ResumeProgress{Record: toApplicationRecord(next), Execution: *next.Execution, Pending: true}, nil
}

func (r *ResumeRepository) LoadIntent(_ context.Context, progress leaseapp.ResumeProgress) (leaseapp.ResumeIntentState, error) {
	if r == nil || r.store == nil || progress.Execution.Pending == nil {
		return leaseapp.ResumeIntentState{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("resume pending intent is required"))
	}
	recordRaw, ok, err := r.store.Get(recordBucket, progress.Record.ID)
	if err != nil {
		return leaseapp.ResumeIntentState{}, err
	}
	if !ok {
		return leaseapp.ResumeIntentState{}, fmt.Errorf("issueops record %s not found", progress.Record.ID)
	}
	record, err := decodeMutableLeaseRecord(progress.Record.ID, recordRaw)
	if err != nil {
		return leaseapp.ResumeIntentState{}, err
	}
	operationID := progress.Execution.Pending.OperationID
	intentRaw, ok, err := r.store.Get("external_intent_v1", operationID)
	if err != nil {
		return leaseapp.ResumeIntentState{}, err
	}
	if !ok {
		return leaseapp.ResumeIntentState{}, fmt.Errorf("Orca external intent payload is missing")
	}
	intent, err := (preparationcontract.IntentCodec{}).Decode(operationID, intentRaw)
	if err != nil {
		return leaseapp.ResumeIntentState{}, err
	}
	if err := preparationdomain.ValidateIntentRecord(record, intent); err != nil {
		return leaseapp.ResumeIntentState{}, err
	}
	return leaseapp.ResumeIntentState{
		Progress:    leaseapp.ResumeProgress{Record: toApplicationRecord(record), Execution: *record.Execution, Pending: record.Execution.Pending != nil},
		OperationID: operationID, Stage: string(intent.Stage), InvocationState: intent.InvocationState,
		InvocationAttempts: intent.InvocationAttempts, RecordRaw: append([]byte(nil), recordRaw...), IntentRaw: append([]byte(nil), intentRaw...),
	}, nil
}

func (r *ResumeRepository) MarkInvoking(ctx context.Context, intent leaseapp.ResumeIntentState) (leaseapp.ResumeIntentState, error) {
	if r == nil || r.store == nil {
		return leaseapp.ResumeIntentState{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("transactional record store is required"))
	}
	if _, ok := r.store.(port.RecordRawCASStore); !ok {
		return leaseapp.ResumeIntentState{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("resume record store does not support raw CAS"))
	}
	updated, data, err := markOrcaIntentInvoking(ctx, r.store, intent.Progress.Record.Stable, intent.OperationID, intent.RecordRaw, intent.IntentRaw)
	if err != nil {
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
	return recordOrcaIntentFailure(ctx, store, orcaFailureState{
		Record: intent.Progress.Record.Stable, RecordRaw: intent.RecordRaw,
		IntentRaw: intent.IntentRaw, OperationID: intent.OperationID,
	}, invocation, cause, r.now, r.redact)
}

func (r *ResumeRepository) ApplyReceipt(ctx context.Context, intent leaseapp.ResumeIntentState, receipt leasecontract.ResumeStageReceipt) (leaseapp.ResumeProgress, error) {
	if err := recordcodec.RequireMutableLeaseSnapshot(intent.Progress.Record.Stable, intent.RecordRaw); err != nil {
		return leaseapp.ResumeProgress{}, err
	}
	if r == nil || r.store == nil {
		return leaseapp.ResumeProgress{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("transactional record store is required"))
	}
	store, ok := r.store.(port.RecordRawCASStore)
	if !ok {
		return leaseapp.ResumeProgress{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("resume record store does not support raw CAS"))
	}
	payload, err := (preparationcontract.IntentCodec{}).Decode(intent.OperationID, intent.IntentRaw)
	if err != nil {
		return leaseapp.ResumeProgress{}, err
	}
	var deliveryErr error
	if payload.Stage == preparationcontract.IntentStageDispatch {
		deliveryErr = port.ValidateExecutionOrcaDeliveryReceipt(resumePortReceipt(receipt), port.OrcaDeliveryReceiptExpectation{
			Host: payload.Probe.Host, TaskID: payload.TaskID, TerminalPTYID: payload.TerminalPTYID,
			DispatchRequestID: payload.OrcaRequestID, PromptRequestID: payload.OrcaPromptRequestID,
		})
	}
	decision, err := preparationdomain.ApplyResumeReceipt(intent.Progress.Record.Stable, payload, resumePreparationReceipt(receipt), deliveryErr)
	if err != nil {
		return leaseapp.ResumeProgress{}, err
	}
	recordData, err := recordcodec.EncodeLease(decision.Record)
	if err != nil {
		return leaseapp.ResumeProgress{}, err
	}
	mutations := []port.RecordMutation{{Bucket: recordBucket, ID: decision.Record.ID, Data: recordData}}
	if decision.Complete {
		mutations = append(mutations, port.RecordMutation{Bucket: "external_intent_v1", ID: intent.OperationID, Delete: true})
	} else {
		intentData, err := (preparationcontract.IntentCodec{}).Encode(decision.Intent)
		if err != nil {
			return leaseapp.ResumeProgress{}, err
		}
		mutations = append(mutations, port.RecordMutation{Bucket: "external_intent_v1", ID: intent.OperationID, Data: intentData})
	}
	err = store.CompareAndApply(ctx, []port.ExpectedRecord{
		{Bucket: recordBucket, ID: decision.Record.ID, Data: intent.RecordRaw},
		{Bucket: "external_intent_v1", ID: intent.OperationID, Data: intent.IntentRaw},
	}, mutations)
	if stale, ok := errors.AsType[port.RawCASFailure](err); ok {
		if stale.FailedBucket() == recordBucket {
			return leaseapp.ResumeProgress{}, fmt.Errorf("stale raw record snapshot")
		}
		return leaseapp.ResumeProgress{}, fmt.Errorf("stale raw intent snapshot")
	}
	if err != nil {
		return leaseapp.ResumeProgress{}, err
	}
	return leaseapp.ResumeProgress{Record: toApplicationRecord(decision.Record), Execution: *decision.Record.Execution, Pending: !decision.Complete}, nil
}

func resumePreparationReceipt(receipt leasecontract.ResumeStageReceipt) preparationcontract.IntentReceipt {
	result := preparationcontract.IntentReceipt{
		TerminalPTYID: receipt.TerminalPTYID, TerminalHandle: receipt.TerminalHandle,
		RunID: receipt.RunID, RunBound: receipt.RunBound, TaskID: receipt.TaskID,
		DispatchID: receipt.DispatchID, RequestID: receipt.RequestID,
	}
	if receipt.PromptReceipt != nil {
		result.PromptReceipt = &preparationcontract.PromptReceipt{
			RequestID: receipt.PromptReceipt.RequestID, Stages: append([]string(nil), receipt.PromptReceipt.Stages...),
			Provider: receipt.PromptReceipt.Provider, Observation: receipt.PromptReceipt.Observation,
			ProcessIncarnation: receipt.PromptReceipt.ProcessIncarnation, Generation: receipt.PromptReceipt.Generation,
			BaselineWorkingSequence: receipt.PromptReceipt.BaselineWorkingSequence,
		}
	}
	return result
}

func resumePortReceipt(receipt leasecontract.ResumeStageReceipt) port.ExecutionOrcaIntentReceipt {
	result := port.ExecutionOrcaIntentReceipt{
		TerminalPTYID: receipt.TerminalPTYID, TerminalHandle: receipt.TerminalHandle,
		RunID: receipt.RunID, RunBound: receipt.RunBound, TaskID: receipt.TaskID,
		DispatchID: receipt.DispatchID, RequestID: receipt.RequestID,
	}
	if receipt.PromptReceipt != nil {
		result.PromptReceipt = &port.OrcaPromptReceipt{
			RequestID: receipt.PromptReceipt.RequestID, Stages: append([]string(nil), receipt.PromptReceipt.Stages...),
			Provider: receipt.PromptReceipt.Provider, Observation: receipt.PromptReceipt.Observation,
			ProcessIncarnation: receipt.PromptReceipt.ProcessIncarnation, Generation: receipt.PromptReceipt.Generation,
			BaselineWorkingSequence: receipt.PromptReceipt.BaselineWorkingSequence,
		}
	}
	return result
}
