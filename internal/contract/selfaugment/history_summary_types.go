package selfaugment

import (
	"issueops/internal/contract/failurecause"
	selfverifycontract "issueops/internal/contract/selfverify"
)

type SelfAugmentSummary struct {
	TotalRuns            int                                                 `json:"total_runs"`
	TotalSteps           int                                                 `json:"total_steps"`
	PassedSteps          int                                                 `json:"passed_steps"`
	FailedSteps          int                                                 `json:"failed_steps"`
	TargetScore          float64                                             `json:"target_score"`
	Contract             selfverifycontract.SelfVerificationContract         `json:"contract"`
	MinimumGoalScore     float64                                             `json:"minimum_goal_score"`
	TerminationEligible  bool                                                `json:"termination_eligible"`
	GoalScores           []selfverifycontract.SelfVerificationGoalScore      `json:"goal_scores"`
	Coverage             []selfverifycontract.SelfVerificationCoverage       `json:"coverage"`
	CoverageGaps         []string                                            `json:"coverage_gaps"`
	RerunCommands        []string                                            `json:"rerun_commands,omitempty"`
	FailureClass         string                                              `json:"failure_class,omitempty"`
	FailureClassReason   string                                              `json:"failure_class_reason,omitempty"`
	FailureCause         failurecause.Cause                                  `json:"failure_cause"`
	FailureCauseReason   string                                              `json:"failure_cause_reason"`
	FailureCauseEvidence []failurecause.Evidence                             `json:"failure_cause_evidence"`
	FailureClusters      []selfverifycontract.SelfVerificationFailureCluster `json:"failure_clusters,omitempty"`
	FailedIteration      int                                                 `json:"failed_iteration,omitempty"`
	FailedSeed           int64                                               `json:"failed_seed,omitempty"`
	FailedStep           string                                              `json:"failed_step,omitempty"`
	StepLabels           []string                                            `json:"step_labels"`
	SlowestSteps         []SelfAugmentSlowStep                               `json:"slowest_steps"`
	StepDurationStats    []SelfAugmentStepDurationStat                       `json:"step_duration_stats"`
}

type SelfAugmentSlowStep struct {
	Iteration  int    `json:"iteration"`
	Seed       int64  `json:"seed"`
	Label      string `json:"label"`
	DurationMS int64  `json:"duration_ms"`
}

type SelfAugmentStepDurationStat struct {
	Label             string  `json:"label"`
	Count             int     `json:"count"`
	ReusedCount       int     `json:"reused_count"`
	MinDurationMS     int64   `json:"min_duration_ms"`
	MaxDurationMS     int64   `json:"max_duration_ms"`
	AverageDurationMS float64 `json:"average_duration_ms"`
	P95DurationMS     int64   `json:"p95_duration_ms"`
}
