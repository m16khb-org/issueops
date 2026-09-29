package audit

import (
	auditcontract "issueops/internal/contract/audit"
	issueopscontract "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
)

func AuditHandoffDeliveryObservation(observation issueopscontract.IssueOpsHandoffDeliveryObservation) (auditcontract.HandoffDeliveryAuditRecord, error) {
	return AuditHandoffDeliveryObservationAt(StateDir(), observation)
}

func ReadHandoffDeliveryAuditObservations() ([]issueopscontract.IssueOpsHandoffDeliveryObservation, error) {
	return ReadHandoffDeliveryAuditObservationsAt(StateDir())
}

func FoldHandoffDeliveryAuditObservations() (map[string]issueopscontract.IssueOpsHandoffDeliveryObservation, []issueopscontract.IssueOpsHandoffDeliveryDecision, error) {
	return FoldHandoffDeliveryAuditObservationsAt(StateDir())
}

func FoldHandoffDeliveryAuditObservationsAt(stateRoot string) (map[string]issueopscontract.IssueOpsHandoffDeliveryObservation, []issueopscontract.IssueOpsHandoffDeliveryDecision, error) {
	observations, err := ReadHandoffDeliveryAuditObservationsAt(stateRoot)
	folded, decisions := issueopsdomain.FoldHandoffDeliveryObservations(observations)
	return folded, decisions, err
}

func FoldHandoffDeliveryAuditObservationsForAt(stateRoot, lifecycleID, lineageID string) (map[string]issueopscontract.IssueOpsHandoffDeliveryObservation, []issueopscontract.IssueOpsHandoffDeliveryDecision, error) {
	observations, err := readHandoffDeliveryAuditObservationsAt(stateRoot, lifecycleID, lineageID)
	folded, decisions := issueopsdomain.FoldHandoffDeliveryObservations(observations)
	return folded, decisions, err
}
