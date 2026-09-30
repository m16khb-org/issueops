package issueops

const (
	IssueOpsSchemaVersion       = 1
	OrcaArtifactIdentityVersion = 1
)

type ExecutionMode string

const (
	ExecutionModeDirect ExecutionMode = "direct"
	ExecutionModeOrca   ExecutionMode = "orca"
)

type LeaseStatus string

const (
	LeaseStatusClaimable LeaseStatus = "claimable"
	LeaseStatusActive    LeaseStatus = "active"
	LeaseStatusRevoking  LeaseStatus = "revoking"
	LeaseStatusReleased  LeaseStatus = "released"
)

type Execution struct {
	Mode               ExecutionMode                `json:"mode"`
	Selection          *ExecutionSelection          `json:"selection,omitempty"`
	Workspace          Workspace                    `json:"workspace"`
	Lease              WriteLease                   `json:"lease"`
	Orca               *OrcaBinding                 `json:"orca,omitempty"`
	Pending            *ExternalIntent              `json:"pending,omitempty"`
	Completion         *ExecutionCompletion         `json:"completion,omitempty"`
	CompletionHistory  []ExecutionCompletionHistory `json:"completion_history,omitempty"`
	Failure            *ExecutionFailure            `json:"failure,omitempty"`
	SyncBaseResolution *ExecutionSyncBaseResolution `json:"sync_base_resolution,omitempty"`
	// SyncBaseEvents는 completion 이후 base 재동기화(merge+push)의 durable
	// 감사 기록이다. append-only이며 Completion.FinalHead는 불변으로 남는다
	// — 완결 시점 증거를 보존하고, PR head는 provider가 관측하며, merge OID는
	// 이 이벤트가 담당한다(설계 v2 design-review F9). 기존 레코드는 nil이므로
	// 스키마는 additive다.
	SyncBaseEvents []ExecutionSyncBaseEvent `json:"sync_base_events,omitempty"`
}

type ExecutionSelection struct {
	RequestedMode        string `json:"requested_mode"`
	ResolvedMode         string `json:"resolved_mode"`
	ProbeAttempted       bool   `json:"probe_attempted"`
	ProbeAvailable       bool   `json:"probe_available"`
	ProbeReady           bool   `json:"probe_ready"`
	ProbeCode            string `json:"probe_code,omitempty"`
	FallbackCode         string `json:"fallback_code,omitempty"`
	ReadinessFingerprint string `json:"readiness_fingerprint"`
	SelectedAt           string `json:"selected_at"`
	ExplicitDirectReason string `json:"explicit_direct_reason,omitempty"`
}

type Workspace struct {
	SourceRoot     string `json:"source_root"`
	Root           string `json:"root"`
	Branch         string `json:"branch"`
	BaseHead       string `json:"base_head"`
	ParentWorktree string `json:"parent_worktree,omitempty"`
	Driver         string `json:"driver"`
	LinkedAt       string `json:"linked_at"`
	// ArtifactDir은 봉인 아티팩트(plan/spec/verified-execution-loop)를 materialize·재검증·
	// 게시할 워크트리 상대 디렉터리다(slash 구분, #482). 비어 있으면 legacy
	// `.issueops/artifact`를 뜻한다.
	ArtifactDir string `json:"artifact_dir,omitempty"`
}

type WriteLease struct {
	Generation        uint64       `json:"generation"`
	Status            LeaseStatus  `json:"status"`
	Holder            *NativeActor `json:"holder,omitempty"`
	ClaimTokenSHA256  string       `json:"claim_token_sha256,omitempty"`
	ClaimedAt         string       `json:"claimed_at,omitempty"`
	ReleasedAt        string       `json:"released_at,omitempty"`
	ReplacedAt        string       `json:"replaced_at,omitempty"`
	ReplacementReason string       `json:"replacement_reason,omitempty"`
}

type NativeActor struct {
	Host           string                `json:"host"`
	SessionID      string                `json:"session_id"`
	AgentID        string                `json:"agent_id,omitempty"`
	SessionProcess *NativeProcessReceipt `json:"session_process,omitempty"`
	// ProcessAncestry is populated by a first-party adapter from the local OS
	// process tree. It is never accepted from JSON or persisted.
	ProcessAncestry []NativeProcessReceipt `json:"-"`
}

type NativeProcessReceipt struct {
	PID        int    `json:"pid"`
	StartedAt  string `json:"started_at"`
	Executable string `json:"executable"`
}

type OrcaBinding struct {
	RuntimeID               string `json:"runtime_id"`
	RepoID                  string `json:"repo_id"`
	WorktreeID              string `json:"worktree_id"`
	RunID                   string `json:"run_id,omitempty"`
	WorktreeInstanceID      string `json:"worktree_instance_id,omitempty"`
	LeaseGeneration         uint64 `json:"lease_generation,omitempty"`
	ArtifactIdentityVersion uint64 `json:"artifact_identity_version,omitempty"`
	IssueBodySHA256         string `json:"issue_body_sha256,omitempty"`
	ContextPacketSHA256     string `json:"context_packet_sha256,omitempty"`
	OwnerPromptSHA256       string `json:"owner_prompt_sha256,omitempty"`
	OwnerHost               string `json:"owner_host"`
	OwnerModel              string `json:"owner_model"`
	OwnerEffort             string `json:"owner_effort,omitempty"`
	TaskID                  string `json:"task_id"`
	DispatchID              string `json:"dispatch_id"`
	TerminalPTYID           string `json:"terminal_pty_id,omitempty"`
}

type ExternalIntent struct {
	OperationID string `json:"operation_id"`
	Kind        string `json:"kind"`
	Marker      string `json:"marker"`
	StartedAt   string `json:"started_at"`
}

type ExecutionCompletion struct {
	Generation             uint64   `json:"generation,omitempty"`
	FinalHead              string   `json:"final_head"`
	VerificationReportPath string   `json:"verification_report_path"`
	Verification           []string `json:"verification"`
	RemoteArtifactURL      string   `json:"remote_artifact_url"`
	CompletedAt            string   `json:"completed_at"`
}

type ExecutionCompletionHistory struct {
	Generation uint64              `json:"generation"`
	Completion ExecutionCompletion `json:"completion"`
	Reason     string              `json:"reason"`
	ReopenedAt string              `json:"reopened_at"`
}

type ExecutionFailure struct {
	OperationID string `json:"operation_id,omitempty"`
	Code        string `json:"code"`
	Message     string `json:"message,omitempty"`
	At          string `json:"at"`
}

const (
	ExecutionSyncBaseEventApply    = "apply"
	ExecutionSyncBaseEventFinalize = "finalize"
)

// ExecutionSyncBaseEvent는 성공한 sync-base 변형 1회의 증거다. abort는
// 되돌림이므로 이벤트를 남기지 않는다(설계 v2 — apply/finalize 성공 시에만 append).
type ExecutionSyncBaseEvent struct {
	Mode          string `json:"mode"` // apply | finalize
	BaseBranch    string `json:"base_branch"`
	BaseOID       string `json:"base_oid"`
	MergeCommit   string `json:"merge_commit"`
	ConflictFiles int    `json:"conflict_files"`
	Actor         string `json:"actor"`
	At            string `json:"at"`
}

// ExecutionSyncBaseResolution seals temporary conflict-resolution authority
// without reopening the released completion as a general write lease.
type ExecutionSyncBaseResolution struct {
	Generation           uint64      `json:"generation"`
	CompletionGeneration uint64      `json:"completion_generation"`
	BaseOID              string      `json:"base_oid"`
	Actor                NativeActor `json:"actor"`
	ConflictFiles        []string    `json:"conflict_files"`
	StartedAt            string      `json:"started_at"`
}
