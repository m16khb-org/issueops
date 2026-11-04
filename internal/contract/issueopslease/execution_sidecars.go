package issueopslease

// 아래 타입은 lease vertical이 typed로 다루는 execution sidecar다. 나머지 record
// field는 Record의 json.RawMessage로 원문 그대로 보존한다. production execution의
// field가 여기서 빠지면 Encode가 그 field를 버리므로,
// TestLeaseExecutionShapeCoversEveryPersistedExecutionField가 두 shape를 대조한다.

// ExternalIntent 타입은 execution이 아직 해소하지 못한 외부 작업(pending)의 종류와 식별자(OperationID, Marker)다.
// 해소되기 전에는 cleanup abandon 같은 후속 작업을 막는 근거로 쓰인다.
type ExternalIntent struct {
	OperationID string `json:"operation_id"`
	Kind        string `json:"kind"`
	Marker      string `json:"marker"`
	StartedAt   string `json:"started_at"`
}

// Completion 타입은 execution 완료 시점에 기록하는 최종 head, 검증 보고서 경로, 검증 명령과 원격 artifact URL이다.
type Completion struct {
	Generation             uint64   `json:"generation,omitempty"`
	FinalHead              string   `json:"final_head"`
	VerificationReportPath string   `json:"verification_report_path"`
	Verification           []string `json:"verification"`
	RemoteArtifactURL      string   `json:"remote_artifact_url"`
	CompletedAt            string   `json:"completed_at"`
}

// CompletionHistoryEntry 타입은 reopen으로 대체된 이전 완료 기록과 reopen 사유·시각을 한 건으로 묶는다.
type CompletionHistoryEntry struct {
	Generation uint64     `json:"generation"`
	Completion Completion `json:"completion"`
	Reason     string     `json:"reason"`
	ReopenedAt string     `json:"reopened_at"`
}

// FailureDetail 타입은 execution이 마지막으로 기록한 외부 작업 실패의 식별자, 코드, 메시지와 시각이다.
type FailureDetail struct {
	OperationID string `json:"operation_id,omitempty"`
	Code        string `json:"code"`
	Message     string `json:"message,omitempty"`
	At          string `json:"at"`
}

// SyncBaseResolution 타입은 base 브랜치 병합이 충돌로 멈췄을 때 해결 중인 base OID, 담당 actor, 충돌 파일을 기록한다.
type SyncBaseResolution struct {
	Generation           uint64   `json:"generation"`
	CompletionGeneration uint64   `json:"completion_generation"`
	BaseOID              string   `json:"base_oid"`
	Actor                Actor    `json:"actor"`
	ConflictFiles        []string `json:"conflict_files"`
	StartedAt            string   `json:"started_at"`
}

// SyncBaseEvent 타입은 sync-base 한 번의 실행 결과(모드, base, 병합 커밋, 충돌 파일 수, 실행자)를 남기는 이력 항목이다.
type SyncBaseEvent struct {
	Mode          string `json:"mode"`
	BaseBranch    string `json:"base_branch"`
	BaseOID       string `json:"base_oid"`
	MergeCommit   string `json:"merge_commit"`
	ConflictFiles int    `json:"conflict_files"`
	Actor         string `json:"actor"`
	At            string `json:"at"`
}
