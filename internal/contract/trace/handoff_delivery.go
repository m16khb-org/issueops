package trace

import issueopscontract "issueops/internal/contract/issueops"

type HandoffDeliveryObserveResult struct {
	OK          bool                                                `json:"ok"`
	Kind        string                                              `json:"kind"`
	AuditLogID  string                                              `json:"audit_log_id"`
	Observation issueopscontract.IssueOpsHandoffDeliveryObservation `json:"observation"`
}
