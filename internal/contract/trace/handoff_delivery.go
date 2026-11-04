package trace

import issueopscontract "issueops/internal/contract/issueops"

// HandoffDeliveryObserveResult is the JSON result of observing a handoff delivery:
// the record's kind and audit log ID plus the delivery observation that was recorded.
type HandoffDeliveryObserveResult struct {
	OK          bool                                        `json:"ok"`
	Kind        string                                      `json:"kind"`
	AuditLogID  string                                      `json:"audit_log_id"`
	Observation issueopscontract.HandoffDeliveryObservation `json:"observation"`
}
