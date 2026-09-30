package audit

import issueopscontract "issueops/internal/contract/issueops"

type HandoffDeliveryAuditRecord struct {
	OK            bool                                                `json:"ok"`
	Kind          string                                              `json:"kind"`
	SchemaVersion int                                                 `json:"schema_version"`
	AuditLogID    string                                              `json:"audit_log_id"`
	GeneratedAt   string                                              `json:"generated_at"`
	LogPath       string                                              `json:"log_path,omitempty"`
	RecordDigest  string                                              `json:"record_digest"`
	Observation   issueopscontract.IssueOpsHandoffDeliveryObservation `json:"observation"`
}
