package selfaugment

import (
	"issueops/internal/contract/failurecause"
	selfverifycontract "issueops/internal/contract/selfverify"
)

// Summary 타입은 자기 검증 루프 실행 결과의 집계 요약이다.
// 단계 통과/실패 수, 목표별 점수, 실패 원인 분류, 느린 단계 통계를 담고 종료 가능 여부 판단의 근거가 된다.
type Summary struct {
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
	PlannedSteps         int                                                 `json:"planned_steps"`
	NotRunSteps          []string                                            `json:"not_run_steps"`
	SlowestSteps         []SlowStep                                          `json:"slowest_steps"`
	StepDurationStats    []StepDurationStat                                  `json:"step_duration_stats"`
}

// SlowStep 타입은 한 반복에서 측정된 단계 하나의 소요 시간(밀리초)을 기록한다.
type SlowStep struct {
	Iteration  int    `json:"iteration"`
	Seed       int64  `json:"seed"`
	Label      string `json:"label"`
	DurationMS int64  `json:"duration_ms"`
}

// StepDurationStat 타입은 단계 레이블별 소요 시간(밀리초) 통계이다.
// 실행 횟수, 재사용된 횟수, 최소/최대/평균/p95 값을 담는다.
type StepDurationStat struct {
	Label             string  `json:"label"`
	Count             int     `json:"count"`
	ReusedCount       int     `json:"reused_count"`
	MinDurationMS     int64   `json:"min_duration_ms"`
	MaxDurationMS     int64   `json:"max_duration_ms"`
	AverageDurationMS float64 `json:"average_duration_ms"`
	P95DurationMS     int64   `json:"p95_duration_ms"`
}
