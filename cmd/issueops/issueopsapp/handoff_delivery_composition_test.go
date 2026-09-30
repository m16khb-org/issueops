package issueopsapp

import (
	"time"

	deliveryapp "issueops/internal/application/handoffdelivery"
	issueopscontract "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

func observeHandoffDeliveryStaged(stateRoot string, request port.ExecutionOrcaIntentRequest, identity port.ExecutionOrcaDeliveryIdentity, callKind string, receipt port.ExecutionOrcaIntentReceipt, now func() time.Time) error {
	return deliveryapp.ObserveStaged(newHandoffDeliveryAudit(stateRoot), request, identity, callKind, receipt, now)
}
func observeHandoffDeliveryCompleted(stateRoot string, request port.ExecutionOrcaIntentRequest, identity port.ExecutionOrcaDeliveryIdentity, callKind string, receipt port.ExecutionOrcaIntentReceipt, now func() time.Time) error {
	return deliveryapp.ObserveCompleted(newHandoffDeliveryAudit(stateRoot), request, identity, callKind, receipt, now)
}
func observeHandoffDeliveryFailure(stateRoot string, request port.ExecutionOrcaIntentRequest, identity port.ExecutionOrcaDeliveryIdentity, err error, now func() time.Time) error {
	return deliveryapp.ObserveFailure(newHandoffDeliveryAudit(stateRoot), request, identity, err, now)
}
func handoffDeliveryObservation(stateRoot string, request port.ExecutionOrcaIntentRequest, identity port.ExecutionOrcaDeliveryIdentity, receipt port.ExecutionOrcaIntentReceipt, durableID, callKind string, now func() time.Time) (issueopscontract.IssueOpsHandoffDeliveryObservation, error) {
	return deliveryapp.Observation(newHandoffDeliveryAudit(stateRoot), request, identity, receipt, durableID, callKind, now)
}
func handoffDeliveryEventClock(now func() time.Time) func() time.Time {
	timestamp := now()
	return func() time.Time { return timestamp }
}
func handoffDeliveryObserved(now func() time.Time, evidence string) issueopscontract.IssueOpsHandoffDeliveryState {
	return issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateObserved, ObservedAt: now().UTC().Format(time.RFC3339Nano), Evidence: evidence}
}

func foldHandoffAuditForTest(stateRoot, lifecycleID, lineageID string) (map[string]issueopscontract.IssueOpsHandoffDeliveryObservation, []issueopscontract.IssueOpsHandoffDeliveryDecision, error) {
	observations, err := (newHandoffDeliveryAudit(stateRoot)).ReadFor(lifecycleID, lineageID)
	folded, decisions := issueopsdomain.FoldHandoffDeliveryObservations(observations)
	return folded, decisions, err
}
