// Package issueopslease 패키지는 execution lease가 읽고 쓰는 issueops record v1의 persisted shape를 정의한다.
// 이 vertical은 lease가 typed로 다루는 execution sidecar만 해석하고 나머지 field는
// 원문 그대로 보존하며, JSON 해석과 production DTO 변환만 맡는 contract 계층이다.
package issueopslease

import (
	"bytes"
	"encoding/json"
	"io"

	issueopscontract "issueops/internal/contract/issueops"
	statecontract "issueops/internal/contract/state"
)

// SchemaVersion 상수는 이 vertical이 읽고 쓰는 issueops record의 schema 버전이다.
// Decode와 Encode는 이 값과 다른 record를 invalid로 거부한다.
const SchemaVersion = 1

// Record 타입은 release가 변경하지 않는 v1 sidecar를 canonical DTO로 보존한다.
type Record struct {
	OK                      bool                             `json:"ok"`
	SchemaVersion           int                              `json:"schema_version"`
	ID                      string                           `json:"id"`
	Repo                    string                           `json:"repo"`
	Branch                  string                           `json:"branch,omitempty"`
	Phase                   string                           `json:"phase"`
	Intent                  json.RawMessage                  `json:"intent,omitempty"`
	ArchitectureReview      json.RawMessage                  `json:"architecture_review,omitempty"`
	DomainReview            json.RawMessage                  `json:"domain_review,omitempty"`
	IssueURL                string                           `json:"issue_url,omitempty"`
	ChildCreateOperations   json.RawMessage                  `json:"child_create_operations,omitempty"`
	IssueCreateIntent       json.RawMessage                  `json:"issue_create_intent,omitempty"`
	PlanPath                string                           `json:"plan_path,omitempty"`
	WorktreePath            string                           `json:"worktree_path,omitempty"`
	IssueLinks              json.RawMessage                  `json:"issue_links,omitempty"`
	BranchPrepare           json.RawMessage                  `json:"branch_prepare,omitempty"`
	RemoteArtifact          json.RawMessage                  `json:"remote_artifact,omitempty"`
	BodySyncs               json.RawMessage                  `json:"body_syncs,omitempty"`
	Decisions               json.RawMessage                  `json:"decisions,omitempty"`
	PlanPrep                json.RawMessage                  `json:"plan_prep,omitempty"`
	CompatibilityReview     json.RawMessage                  `json:"compatibility_review,omitempty"`
	DevilsAdvocateReview    json.RawMessage                  `json:"devils_advocate_review,omitempty"`
	Feedback                json.RawMessage                  `json:"feedback,omitempty"`
	RegressEvents           json.RawMessage                  `json:"regress_events,omitempty"`
	Delegation              json.RawMessage                  `json:"delegation,omitempty"`
	ChildCycles             json.RawMessage                  `json:"child_cycles,omitempty"`
	Execution               *Execution                       `json:"execution,omitempty"`
	RemoteCompletion        json.RawMessage                  `json:"remote_completion,omitempty"`
	SourceMisdirectWarnings int                              `json:"source_misdirect_warnings,omitempty"`
	CleanupAttempt          *issueopscontract.CleanupAttempt `json:"cleanup_attempt,omitempty"`
	CleanupFinishFailure    json.RawMessage                  `json:"cleanup_finish_failure,omitempty"`
	LinkedBranchCleanup     json.RawMessage                  `json:"linked_branch_cleanup,omitempty"`
	CleanupAbandonFailure   json.RawMessage                  `json:"cleanup_abandon_failure,omitempty"`
	ImplementationReview    json.RawMessage                  `json:"implementation_review,omitempty"`
	ProjectDocsReview       json.RawMessage                  `json:"project_docs_review,omitempty"`
	SchemaEvidence          json.RawMessage                  `json:"schema_evidence,omitempty"`
	RoutingTrace            json.RawMessage                  `json:"routing_trace,omitempty"`
	AISlopCleanAt           string                           `json:"ai_slop_clean_at,omitempty"`
	AISlopCleanHead         string                           `json:"ai_slop_clean_head,omitempty"`
	AISlopCleanFingerprint  string                           `json:"ai_slop_clean_fingerprint,omitempty"`
	AISlopCleanCategories   json.RawMessage                  `json:"ai_slop_clean_categories,omitempty"`
	AISlopCleanVerification json.RawMessage                  `json:"ai_slop_clean_verification,omitempty"`
	PRHead                  string                           `json:"pr_head,omitempty"`
	PhaseLedger             json.RawMessage                  `json:"phase_ledger,omitempty"`
	CreatedAt               string                           `json:"created_at"`
	UpdatedAt               string                           `json:"updated_at"`
}

// Execution 타입은 record의 execution 섹션 중 lease vertical이 typed로 다루는 workspace, lease, 완료·실패·sync-base sidecar다.
type Execution struct {
	Workspace          Workspace                `json:"workspace"`
	Lease              Lease                    `json:"lease"`
	Pending            *ExternalIntent          `json:"pending,omitempty"`
	Completion         *Completion              `json:"completion,omitempty"`
	CompletionHistory  []CompletionHistoryEntry `json:"completion_history,omitempty"`
	Failure            *FailureDetail           `json:"failure,omitempty"`
	SyncBaseResolution *SyncBaseResolution      `json:"sync_base_resolution,omitempty"`
	SyncBaseEvents     []SyncBaseEvent          `json:"sync_base_events,omitempty"`
}

// Workspace 타입은 execution이 묶인 작업 worktree의 위치(source root, root, branch, base head, artifact 디렉터리)다.
type Workspace struct {
	SourceRoot     string `json:"source_root"`
	Root           string `json:"root"`
	Branch         string `json:"branch"`
	BaseHead       string `json:"base_head"`
	ParentWorktree string `json:"parent_worktree,omitempty"`
	LinkedAt       string `json:"linked_at"`
	ArtifactDir    string `json:"artifact_dir,omitempty"`
}

// Lease 타입은 execution 쓰기 권한의 generation, 상태, 보유자와 claim token 해시, 획득·반납·교체 시각이다.
// 평문 claim token은 저장하지 않고 SHA-256만 보존한다.
type Lease struct {
	Generation        uint64 `json:"generation"`
	Status            string `json:"status"`
	Holder            *Actor `json:"holder,omitempty"`
	ClaimTokenSHA256  string `json:"claim_token_sha256,omitempty"`
	ClaimedAt         string `json:"claimed_at,omitempty"`
	ReleasedAt        string `json:"released_at,omitempty"`
	ReplacedAt        string `json:"replaced_at,omitempty"`
	ReplacementReason string `json:"replacement_reason,omitempty"`
}

// Actor 타입은 lease를 보유한 native 호스트 세션(host, session, agent)과 선택적 프로세스 영수증이다.
type Actor struct {
	Host           string          `json:"host"`
	SessionID      string          `json:"session_id"`
	AgentID        string          `json:"agent_id,omitempty"`
	SessionProcess *ProcessReceipt `json:"session_process,omitempty"`
}

// ProcessReceipt 타입은 actor 세션 프로세스를 식별하는 PID, 시작 시각, 실행 파일 경로다.
type ProcessReceipt struct {
	PID        int    `json:"pid"`
	StartedAt  string `json:"started_at"`
	Executable string `json:"executable"`
}

// Decode 함수는 persisted v1 JSON을 production record contract로 엄격하게 읽는다. 모르는
// field는 invalid로 거부해, 이 vertical이 모르는 field를 다시 쓰면서 버리지 않게
// 한다. 그다음 production DTO로 canonical JSON을 만들어 Record로 옮긴다.
// 업무 불변식은 outbound issueopsrecord codec이 domain을 통해 검사한다.
func Decode(id string, data []byte) (Record, error) {
	var shape issueopscontract.Record
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&shape); err != nil {
		return Record{}, statecontract.Invalid("")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Record{}, statecontract.Invalid("")
	}
	if shape.SchemaVersion != SchemaVersion || shape.ID != id {
		return Record{}, statecontract.Invalid("")
	}
	if err := issueopscontract.ValidateCleanupAttempt(shape.CleanupAttempt); err != nil {
		return Record{}, statecontract.Invalid("")
	}
	canonical, err := json.Marshal(shape)
	if err != nil {
		return Record{}, statecontract.Invalid("")
	}
	var record Record
	if err := json.Unmarshal(canonical, &record); err != nil {
		return Record{}, statecontract.Invalid("")
	}
	record.OK = true
	return record, nil
}

// Encode 함수는 record를 들여쓰기 JSON으로 직렬화한다. OK를 true로 맞추고, SchemaVersion이 다르거나
// cleanup attempt가 계약에 어긋나면 invalid state 오류를 돌려준다.
func Encode(record Record) ([]byte, error) {
	record.OK = true
	if record.SchemaVersion != SchemaVersion {
		return nil, statecontract.Invalid("")
	}
	if err := issueopscontract.ValidateCleanupAttempt(record.CleanupAttempt); err != nil {
		return nil, statecontract.Invalid("")
	}
	return json.MarshalIndent(record, "", "  ")
}
