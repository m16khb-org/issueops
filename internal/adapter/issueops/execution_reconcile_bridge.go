package issueops

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// ExecutionReconcileIntentState는 새 reconcile vertical이 기존 durable CAS
// primitive를 호출할 때 필요한 최소 호환 상태다.
type ExecutionReconcileIntentState struct {
	Record             issueops.IssueOpsRecord
	RecordRaw          []byte
	IntentRaw          []byte
	OperationID        string
	Stage              port.ExecutionOrcaIntentStage
	InvocationState    string
	InvocationAttempts int
	Pending            bool
}

func ExecutionReconcileIntentRequest(expected ExecutionReconcileIntentState) (port.ExecutionOrcaIntentRequest, error) {
	payload, err := executionReconcileIntentPayload(expected)
	if err != nil {
		return port.ExecutionOrcaIntentRequest{}, err
	}
	if err := validateOrcaIntentExpectedRecord(expected.Record, payload); err != nil {
		return port.ExecutionOrcaIntentRequest{}, err
	}
	return executionOrcaIntentRequest(expected.Record, payload)
}

func ApplyExecutionReconcileIntentReceipt(ctx context.Context, stateRoot string, expected ExecutionReconcileIntentState, receipt port.ExecutionOrcaIntentReceipt, readIssue ExecutionIssueSnapshotReadFunc, now func() time.Time) (ExecutionReconcileIntentState, error) {
	payload, err := executionReconcileIntentPayload(expected)
	if err != nil {
		return ExecutionReconcileIntentState{}, err
	}
	persisted, nextPayload, err := advanceOrcaIntentReceiptWithExpectedRaw(ctx, stateRoot, expected.Record, payload, expected.RecordRaw, expected.IntentRaw, receipt, readIssue, now)
	if err != nil {
		return ExecutionReconcileIntentState{}, err
	}
	if persisted.Execution == nil || persisted.Execution.Pending == nil {
		raw, err := readExecutionResumeRecordRawOnly(stateRoot, persisted.ID)
		if err != nil {
			return ExecutionReconcileIntentState{}, err
		}
		return ExecutionReconcileIntentState{Record: persisted, RecordRaw: raw, OperationID: expected.OperationID}, nil
	}
	return executionReconcileIntentStateFromPayload(stateRoot, persisted, nextPayload)
}

func executionReconcileIntentStateFromPayload(stateRoot string, record issueops.IssueOpsRecord, payload externalOrcaIntentPayload) (ExecutionReconcileIntentState, error) {
	partial := executionReconcileIntentState(record, nil, payload, nil)
	currentRecord, recordRaw, err := readExecutionResumeRecordRaw(stateRoot, record.ID)
	if err != nil {
		return partial, err
	}
	if !reflect.DeepEqual(currentRecord, record) {
		return partial, fmt.Errorf("IssueOps record snapshot changed before reconcile raw capture")
	}
	currentPayload, intentRaw, err := readExecutionResumeIntentRaw(stateRoot, payload.OperationID)
	if err != nil {
		return partial, err
	}
	if !reflect.DeepEqual(currentPayload, payload) {
		return partial, fmt.Errorf("Orca intent snapshot changed before reconcile raw capture")
	}
	return executionReconcileIntentState(record, recordRaw, payload, intentRaw), nil
}

func executionReconcileIntentState(record issueops.IssueOpsRecord, recordRaw []byte, payload externalOrcaIntentPayload, intentRaw []byte) ExecutionReconcileIntentState {
	return ExecutionReconcileIntentState{
		Record: record, RecordRaw: append([]byte(nil), recordRaw...), IntentRaw: append([]byte(nil), intentRaw...),
		OperationID: payload.OperationID, Stage: intentPortStage(payload.Stage), InvocationState: payload.InvocationState,
		InvocationAttempts: payload.InvocationAttempts, Pending: record.Execution != nil && record.Execution.Pending != nil,
	}
}

func executionReconcileIntentPayload(expected ExecutionReconcileIntentState) (externalOrcaIntentPayload, error) {
	return executionResumeIntentPayload(ExecutionResumeIntentState(expected))
}
