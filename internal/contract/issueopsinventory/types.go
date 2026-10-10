package issueopsinventory

import issueopscontract "issueops/internal/contract/issueops"

// domain/issueopsinventory은 이 contract 패키지만 import할 수 있어서(domain은 같은
// capability 계약만 import) 그 domain이 읽는 공용 IssueOps 어휘를 여기서 이 이름으로 둔다.
type Record = issueopscontract.IssueOpsRecord

type ListResult struct {
	OK             bool               `json:"ok"`
	GeneratedAt    string             `json:"generated_at"`
	ScannedRecords int                `json:"scanned_records"`
	ReadErrors     int                `json:"read_errors"`
	UnreadableIDs  []string           `json:"unreadable_ids"`
	Diagnostics    []RecordDiagnostic `json:"diagnostics"`
	Entries        []ListEntry        `json:"entries"`
}

type RecordDiagnostic struct {
	ID   string `json:"id"`
	Code string `json:"code"`
}

type ListEntry struct {
	ID                    string                         `json:"id"`
	Repo                  string                         `json:"repo"`
	Branch                string                         `json:"branch,omitempty"`
	Phase                 issueopscontract.IssueOpsPhase `json:"phase"`
	Mode                  string                         `json:"mode,omitempty"`
	LeaseStatus           string                         `json:"lease_status,omitempty"`
	HolderHost            string                         `json:"holder_host,omitempty"`
	HolderSession         string                         `json:"holder_session,omitempty"`
	OwnerModel            string                         `json:"owner_model,omitempty"`
	WorkspaceRoot         string                         `json:"workspace_root,omitempty"`
	IssueURL              string                         `json:"issue_url,omitempty"`
	RemoteArtifactURL     string                         `json:"remote_artifact_url,omitempty"`
	UpdatedAt             string                         `json:"updated_at,omitempty"`
	PendingKind           string                         `json:"pending_kind,omitempty"`
	PendingSince          string                         `json:"pending_since,omitempty"`
	FailureCode           string                         `json:"failure_code,omitempty"`
	FailureAt             string                         `json:"failure_at,omitempty"`
	CleanupFailureStep    string                         `json:"cleanup_failure_step,omitempty"`
	CleanupFailureAt      string                         `json:"cleanup_failure_at,omitempty"`
	IssueCreateStatus     string                         `json:"issue_create_status,omitempty"`
	Claimable             bool                           `json:"claimable,omitempty"`
	CleanupCandidate      bool                           `json:"cleanup_candidate,omitempty"`
	CompletionUnreflected bool                           `json:"completion_unreflected,omitempty"`
	Invalid               bool                           `json:"invalid,omitempty"`
}

const (
	LeaseStatusClaimable = issueopscontract.LeaseStatusClaimable
	PhaseDone            = issueopscontract.IssueOpsPhaseDone
)
