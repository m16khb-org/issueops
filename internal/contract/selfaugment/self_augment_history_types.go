package selfaugment

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

type HistoryRetentionOptions struct {
	Limit          int
	PruneRequested bool
	Confirm        bool
}

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

type HistorySkipped struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}
