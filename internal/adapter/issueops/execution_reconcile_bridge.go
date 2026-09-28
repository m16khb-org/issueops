package issueops

import (
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

func executionReconcileIntentPayload(expected ExecutionReconcileIntentState) (externalOrcaIntentPayload, error) {
	return executionResumeIntentPayload(ExecutionResumeIntentState(expected))
}
