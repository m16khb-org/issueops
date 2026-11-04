package selfaugment

type CompareResult struct {
	OK                           bool                   `json:"ok"`
	StateDir                     string                 `json:"state_dir"`
	BaselineKey                  string                 `json:"baseline_key"`
	CandidateKey                 string                 `json:"candidate_key"`
	MaxElapsedRegressionPct      float64                `json:"max_elapsed_regression_pct"`
	Regressed                    bool                   `json:"regressed"`
	ElapsedDeltaMS               int64                  `json:"elapsed_delta_ms"`
	ElapsedDeltaPct              float64                `json:"elapsed_delta_pct"`
	FailedStepsDelta             int                    `json:"failed_steps_delta"`
	TotalStepsDelta              int                    `json:"total_steps_delta"`
	BaselineMinimumGoalScore     float64                `json:"baseline_minimum_goal_score"`
	CandidateMinimumGoalScore    float64                `json:"candidate_minimum_goal_score"`
	MissingStepLabels            []string               `json:"missing_step_labels"`
	AddedStepLabels              []string               `json:"added_step_labels"`
	Regressions                  []string               `json:"regressions"`
	Warnings                     []string               `json:"warnings"`
	BaselineSummary              Summary                `json:"baseline_summary"`
	CandidateSummary             Summary                `json:"candidate_summary"`
	BaselineSnapshotGeneratedAt  string                 `json:"baseline_snapshot_generated_at"`
	CandidateSnapshotGeneratedAt string                 `json:"candidate_snapshot_generated_at"`
	BaselineSlowestSteps         []SlowStep             `json:"baseline_slowest_steps"`
	CandidateSlowestSteps        []SlowStep             `json:"candidate_slowest_steps"`
	SlowStepRegressions          []SlowStepRegression   `json:"slow_step_regressions"`
	BaselineStepDurationStats    []StepDurationStat     `json:"baseline_step_duration_stats"`
	CandidateStepDurationStats   []StepDurationStat     `json:"candidate_step_duration_stats"`
	StepBudgetRegressions        []StepBudgetRegression `json:"step_budget_regressions"`
}

type SlowStepRegression struct {
	Label               string  `json:"label"`
	BaselineDurationMS  int64   `json:"baseline_duration_ms"`
	CandidateDurationMS int64   `json:"candidate_duration_ms"`
	DeltaMS             int64   `json:"delta_ms"`
	DeltaPct            float64 `json:"delta_pct"`
}

type StepBudgetRegression struct {
	Label               string  `json:"label"`
	Metric              string  `json:"metric"`
	BaselineDurationMS  int64   `json:"baseline_duration_ms"`
	CandidateDurationMS int64   `json:"candidate_duration_ms"`
	DeltaMS             int64   `json:"delta_ms"`
	DeltaPct            float64 `json:"delta_pct"`
	BaselineCount       int     `json:"baseline_count"`
	CandidateCount      int     `json:"candidate_count"`
}
