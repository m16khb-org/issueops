package model

import (
	"issueops/cmd/issueops/commandstep"
	"issueops/internal/contract/failurecause"
	selfverifycontract "issueops/internal/contract/selfverify"
)

type SelfAugmentResult struct {
	OK                  bool                        `json:"ok"`
	LoopKind            string                      `json:"loop_kind"`
	KoreanName          string                      `json:"korean_name"`
	Iterations          int                         `json:"iterations"`
	BaseSeed            int64                       `json:"base_seed"`
	TargetScore         float64                     `json:"target_score"`
	TerminationEligible bool                        `json:"termination_eligible"`
	ElapsedMS           int64                       `json:"elapsed_ms"`
	IssueOpsRoot        string                      `json:"issueops_root"`
	InspiredBy          string                      `json:"inspired_by"`
	LoopContract        []string                    `json:"loop_contract"`
	Summary             SelfAugmentSummary          `json:"summary"`
	StateCheckpoint     *SelfAugmentStateCheckpoint `json:"state_checkpoint,omitempty"`
	LLMEval             *SelfVerifyLLMEvalResult    `json:"llm_eval,omitempty"`
	Runs                []SelfAugmentIteration      `json:"runs"`
}

type SelfVerifyLLMEvalResult struct {
	OK                     bool     `json:"ok"`
	Mode                   string   `json:"mode"`
	ExecutionClass         string   `json:"execution_class"`
	ReadOnly               bool     `json:"read_only"`
	Score                  float64  `json:"score"`
	Summary                string   `json:"summary,omitempty"`
	Blockers               []string `json:"blockers,omitempty"`
	Risks                  []string `json:"risks,omitempty"`
	RecommendedNextActions []string `json:"recommended_next_actions,omitempty"`
	EvidencePacketBytes    int      `json:"evidence_packet_bytes"`
	Prompt                 string   `json:"prompt,omitempty"`
	Error                  string   `json:"error,omitempty"`
}

type SelfAugmentSummary struct {
	TotalRuns            int                              `json:"total_runs"`
	TotalSteps           int                              `json:"total_steps"`
	PassedSteps          int                              `json:"passed_steps"`
	FailedSteps          int                              `json:"failed_steps"`
	TargetScore          float64                          `json:"target_score"`
	Contract             SelfVerificationContract         `json:"contract"`
	MinimumGoalScore     float64                          `json:"minimum_goal_score"`
	TerminationEligible  bool                             `json:"termination_eligible"`
	GoalScores           []SelfVerificationGoalScore      `json:"goal_scores"`
	Coverage             []SelfVerificationCoverage       `json:"coverage"`
	CoverageGaps         []string                         `json:"coverage_gaps"`
	RerunCommands        []string                         `json:"rerun_commands,omitempty"`
	FailureClass         string                           `json:"failure_class,omitempty"`
	FailureClassReason   string                           `json:"failure_class_reason,omitempty"`
	FailureCause         failurecause.Cause               `json:"failure_cause"`
	FailureCauseReason   string                           `json:"failure_cause_reason"`
	FailureCauseEvidence []failurecause.Evidence          `json:"failure_cause_evidence"`
	FailureClusters      []SelfVerificationFailureCluster `json:"failure_clusters,omitempty"`
	FailedIteration      int                              `json:"failed_iteration,omitempty"`
	FailedSeed           int64                            `json:"failed_seed,omitempty"`
	FailedStep           string                           `json:"failed_step,omitempty"`
	StepLabels           []string                         `json:"step_labels"`
	SlowestSteps         []SelfAugmentSlowStep            `json:"slowest_steps"`
	StepDurationStats    []SelfAugmentStepDurationStat    `json:"step_duration_stats"`
}

type SelfVerificationContract = selfverifycontract.SelfVerificationContract

type SelfVerificationGoalScore = selfverifycontract.SelfVerificationGoalScore

type SelfVerificationCoverage = selfverifycontract.SelfVerificationCoverage

type SelfVerificationFailureCluster = selfverifycontract.SelfVerificationFailureCluster

type SelfAugmentSlowStep struct {
	Iteration  int    `json:"iteration"`
	Seed       int64  `json:"seed"`
	Label      string `json:"label"`
	DurationMS int64  `json:"duration_ms"`
}

type SelfAugmentStepDurationStat struct {
	Label             string  `json:"label"`
	Count             int     `json:"count"`
	MinDurationMS     int64   `json:"min_duration_ms"`
	MaxDurationMS     int64   `json:"max_duration_ms"`
	AverageDurationMS float64 `json:"average_duration_ms"`
	P95DurationMS     int64   `json:"p95_duration_ms"`
}

type SelfVerificationGoalDefinition = selfverifycontract.SelfVerificationGoalDefinition

type SelfVerificationCoverageDefinition = selfverifycontract.SelfVerificationCoverageDefinition

type StepResult = commandstep.StepResult
