// Package issueopsinventory 패키지는 IssueOps cycle 목록 조회 결과의 DTO를
// 소유한다. 항목별 lease, 실패, 정리 후보 신호와 읽지 못한 기록의 진단을 담는
// contract 계층 패키지다.
package issueopsinventory

import issueopscontract "issueops/internal/contract/issueops"

// Record 타입은 공용 IssueOps 기록의 별칭이다. domain/issueopsinventory는 이 contract 패키지만
// import할 수 있어서(domain은 같은 capability 계약만 import) 그 domain이 읽는
// 공용 IssueOps 어휘를 여기서 이 이름으로 둔다.
type Record = issueopscontract.Record

// ListResult 타입은 cycle 목록 조회 결과다. 읽지 못한 기록은 UnreadableIDs와
// Diagnostics에 남고 ReadErrors로 개수를 센다.
type ListResult struct {
	OK             bool               `json:"ok"`
	GeneratedAt    string             `json:"generated_at"`
	ScannedRecords int                `json:"scanned_records"`
	ReadErrors     int                `json:"read_errors"`
	UnreadableIDs  []string           `json:"unreadable_ids"`
	Diagnostics    []RecordDiagnostic `json:"diagnostics"`
	Entries        []ListEntry        `json:"entries"`
}

// RecordDiagnostic 타입은 읽지 못한 기록 한 건의 ID와 원인 코드다.
type RecordDiagnostic struct {
	ID   string `json:"id"`
	Code string `json:"code"`
}

// ListEntry 타입은 cycle 한 건의 목록 행이다. 저장된 기록에서 뽑은 식별과
// lease 정보에 더해 Claimable, CleanupCandidate, CompletionUnreflected,
// Invalid 같은 파생 신호를 담는다.
type ListEntry struct {
	ID                    string                 `json:"id"`
	Repo                  string                 `json:"repo"`
	Branch                string                 `json:"branch,omitempty"`
	Phase                 issueopscontract.Phase `json:"phase"`
	LeaseStatus           string                 `json:"lease_status,omitempty"`
	HolderHost            string                 `json:"holder_host,omitempty"`
	HolderSession         string                 `json:"holder_session,omitempty"`
	WorkspaceRoot         string                 `json:"workspace_root,omitempty"`
	IssueURL              string                 `json:"issue_url,omitempty"`
	RemoteArtifactURL     string                 `json:"remote_artifact_url,omitempty"`
	UpdatedAt             string                 `json:"updated_at,omitempty"`
	LastRecordAt          string                 `json:"last_record_at,omitempty"`
	IdleDays              *int                   `json:"idle_days,omitempty"`
	PendingKind           string                 `json:"pending_kind,omitempty"`
	PendingSince          string                 `json:"pending_since,omitempty"`
	FailureCode           string                 `json:"failure_code,omitempty"`
	FailureAt             string                 `json:"failure_at,omitempty"`
	CleanupFailureStep    string                 `json:"cleanup_failure_step,omitempty"`
	CleanupFailureAt      string                 `json:"cleanup_failure_at,omitempty"`
	IssueCreateStatus     string                 `json:"issue_create_status,omitempty"`
	Claimable             bool                   `json:"claimable,omitempty"`
	CleanupCandidate      bool                   `json:"cleanup_candidate,omitempty"`
	CompletionUnreflected bool                   `json:"completion_unreflected,omitempty"`
	Invalid               bool                   `json:"invalid,omitempty"`
}

// LeaseStatusClaimable과 PhaseDone은 목록 항목의 상태 판정에 쓰는 공용 계약
// 값을 이 패키지 이름으로 다시 노출한다.
const (
	LeaseStatusClaimable = issueopscontract.LeaseStatusClaimable
	PhaseDone            = issueopscontract.PhaseDone
)
