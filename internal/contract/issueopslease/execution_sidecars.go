package issueopslease

// 아래 타입은 lease vertical이 typed로 다루는 execution sidecar다. 나머지 record
// field는 Record의 json.RawMessage로 원문 그대로 보존한다. production execution의
// field가 여기서 빠지면 Encode가 그 field를 버리므로,
// TestLeaseExecutionShapeCoversEveryPersistedExecutionField가 두 shape를 대조한다.

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
