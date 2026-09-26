package model

import (
	"issueops/cmd/issueops/commandstep"
	selfaugmentcontract "issueops/internal/contract/selfaugment"
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

type SelfAugmentSummary = selfaugmentcontract.SelfAugmentSummary

type SelfVerificationContract = selfverifycontract.SelfVerificationContract

type SelfVerificationGoalScore = selfverifycontract.SelfVerificationGoalScore

type SelfVerificationCoverage = selfverifycontract.SelfVerificationCoverage

type SelfVerificationFailureCluster = selfverifycontract.SelfVerificationFailureCluster

type SelfAugmentSlowStep = selfaugmentcontract.SelfAugmentSlowStep

type SelfAugmentStepDurationStat = selfaugmentcontract.SelfAugmentStepDurationStat

type SelfVerificationGoalDefinition = selfverifycontract.SelfVerificationGoalDefinition

type SelfVerificationCoverageDefinition = selfverifycontract.SelfVerificationCoverageDefinition

type StepResult = commandstep.StepResult
