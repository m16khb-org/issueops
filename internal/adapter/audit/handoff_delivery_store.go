package audit

import (
	auditcontract "issueops/internal/contract/audit"
	model "issueops/internal/contract/issueops"
)

type HandoffDeliveryStore struct{ StateRoot string }

func (store HandoffDeliveryStore) Append(observation model.IssueOpsHandoffDeliveryObservation) (auditcontract.HandoffDeliveryAuditRecord, error) {
	return AuditHandoffDeliveryObservationAt(store.StateRoot, observation)
}
func (store HandoffDeliveryStore) Read() ([]model.IssueOpsHandoffDeliveryObservation, error) {
	return ReadHandoffDeliveryAuditObservationsAt(store.StateRoot)
}
func (store HandoffDeliveryStore) ReadFor(lifecycleID, lineageID string) ([]model.IssueOpsHandoffDeliveryObservation, error) {
	return readHandoffDeliveryAuditObservationsAt(store.StateRoot, lifecycleID, lineageID)
}
