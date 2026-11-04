package audit

import issueopscontract "issueops/internal/contract/issueops"

// HandoffDeliveryAuditRecord 타입은 handoff 전달 관찰(Observation)을 append-only 감사 로그에 남긴 기록이다.
// RecordDigest는 기록 내용의 무결성 확인용 다이제스트이고, LogPath는 기록이 쓰인 로그 파일 경로다.
type HandoffDeliveryAuditRecord struct {
	OK            bool                                        `json:"ok"`
	Kind          string                                      `json:"kind"`
	SchemaVersion int                                         `json:"schema_version"`
	AuditLogID    string                                      `json:"audit_log_id"`
	GeneratedAt   string                                      `json:"generated_at"`
	LogPath       string                                      `json:"log_path,omitempty"`
	RecordDigest  string                                      `json:"record_digest"`
	Observation   issueopscontract.HandoffDeliveryObservation `json:"observation"`
}
