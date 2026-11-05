package selfaugment

// HistoryResult 타입은 저장된 자기 검증 요약 체크포인트 이력 조회 결과이다.
// 읽은 항목, 읽지 못해 건너뛴 키, 보존 정책 평가 결과를 담는다.
type HistoryResult struct {
	OK           bool              `json:"ok"`
	StateDir     string            `json:"state_dir"`
	Prefix       string            `json:"prefix"`
	Limit        int               `json:"limit"`
	TotalMatches int               `json:"total_matches"`
	Returned     int               `json:"returned"`
	Retention    *HistoryRetention `json:"retention,omitempty"`
	Entries      []HistoryEntry    `json:"entries"`
	Skipped      []HistorySkipped  `json:"skipped"`
	Warnings     []string          `json:"warnings"`
}

// HistoryRetention 타입은 이력 체크포인트 보존 한도 평가 결과이다.
// 보존할 키와 삭제 후보 키, 실제 삭제된 키, 정리 요청/확인 여부를 담는다.
type HistoryRetention struct {
	Enabled        bool     `json:"enabled"`
	Limit          int      `json:"limit"`
	TotalMatches   int      `json:"total_matches"`
	RetainedKeys   []string `json:"retained_keys"`
	CandidateKeys  []string `json:"candidate_keys"`
	DeletedKeys    []string `json:"deleted_keys"`
	PruneRequested bool     `json:"prune_requested"`
	Confirm        bool     `json:"confirm"`
	DryRun         bool     `json:"dry_run"`
	Recommendation string   `json:"recommendation"`
}

// HistoryRetentionOptions 타입은 이력 보존 평가 옵션이다.
// Limit은 남길 체크포인트 수, PruneRequested는 정리 요청 여부, Confirm은 실제 삭제 확인 여부이다.
type HistoryRetentionOptions struct {
	Limit          int
	PruneRequested bool
	Confirm        bool
}

// HistoryEntry 타입은 이력 조회에서 읽은 체크포인트 하나의 요약 행이다.
type HistoryEntry struct {
	Key          string     `json:"key"`
	UpdatedAt    string     `json:"updated_at"`
	Bytes        int        `json:"bytes"`
	GeneratedAt  string     `json:"generated_at"`
	OK           bool       `json:"ok"`
	Iterations   int        `json:"iterations"`
	BaseSeed     int64      `json:"base_seed"`
	ElapsedMS    int64      `json:"elapsed_ms"`
	TotalRuns    int        `json:"total_runs"`
	TotalSteps   int        `json:"total_steps"`
	FailedSteps  int        `json:"failed_steps"`
	StepLabels   []string   `json:"step_labels"`
	SlowestSteps []SlowStep `json:"slowest_steps"`
}

// HistorySkipped 타입은 이력 조회 중 읽지 못해 건너뛴 체크포인트 키와 그 사유이다.
type HistorySkipped struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}
