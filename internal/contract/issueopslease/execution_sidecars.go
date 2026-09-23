package issueopslease

// 아래 타입은 lease vertical이 typed로 다루는 execution sidecar다. 나머지 record
// field는 Record의 json.RawMessage로 원문 그대로 보존한다. production execution의
// field가 여기서 빠지면 Encode가 그 field를 버리므로,
// TestLeaseExecutionShapeCoversEveryPersistedExecutionField가 두 shape를 대조한다.

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

type Completion struct {
	Generation             uint64   `json:"generation,omitempty"`
	FinalHead              string   `json:"final_head"`
	VerificationReportPath string   `json:"verification_report_path"`
	Verification           []string `json:"verification"`
	RemoteArtifactURL      string   `json:"remote_artifact_url"`
	CompletedAt            string   `json:"completed_at"`
}

type CompletionHistoryEntry struct {
	Generation uint64     `json:"generation"`
	Completion Completion `json:"completion"`
	Reason     string     `json:"reason"`
	ReopenedAt string     `json:"reopened_at"`
}

type FailureDetail struct {
	OperationID string `json:"operation_id,omitempty"`
	Code        string `json:"code"`
	Message     string `json:"message,omitempty"`
	At          string `json:"at"`
}

type SyncBaseResolution struct {
	Generation           uint64   `json:"generation"`
	CompletionGeneration uint64   `json:"completion_generation"`
	BaseOID              string   `json:"base_oid"`
	Actor                Actor    `json:"actor"`
	ConflictFiles        []string `json:"conflict_files"`
	StartedAt            string   `json:"started_at"`
}

type SyncBaseEvent struct {
	Mode          string `json:"mode"`
	BaseBranch    string `json:"base_branch"`
	BaseOID       string `json:"base_oid"`
	MergeCommit   string `json:"merge_commit"`
	ConflictFiles int    `json:"conflict_files"`
	Actor         string `json:"actor"`
	At            string `json:"at"`
}
