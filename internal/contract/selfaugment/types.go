package selfaugment

import qualitycatalog "issueops/internal/contract/qualitycatalog"

type SelfAugmentGoal struct {
	Name        string   `json:"name"`
	KoreanName  string   `json:"korean_name"`
	Score       float64  `json:"score"`
	TargetScore float64  `json:"target_score"`
	Passed      bool     `json:"passed"`
	Description string   `json:"description"`
	Evidence    []string `json:"evidence"`
}

type SelfAugmentCandidate struct {
	ID                   string   `json:"id"`
	Title                string   `json:"title"`
	Category             string   `json:"category"`
	Status               string   `json:"status"`
	Score                float64  `json:"score"`
	Impact               float64  `json:"impact"`
	Feasibility          float64  `json:"feasibility"`
	Novelty              float64  `json:"novelty"`
	Risk                 float64  `json:"risk"`
	WhyNow               []string `json:"why_now"`
	ExpectedGain         []string `json:"expected_gain"`
	VerifyWith           []string `json:"verify_with"`
	SatisfactionEvidence []string `json:"satisfaction_evidence,omitempty"`
	// VerificationKind classifies the candidate's verification (B1). Not
	// serialized: it is catalog-hygiene metadata for the grounding enforcement,
	// not part of the public DTO contract.
	VerificationKind qualitycatalog.VerificationKind `json:"-"`
}

type SelfAugmentRepoSignals struct {
	DocsIndexed                         int      `json:"docs_indexed"`
	Skills                              []string `json:"skills"`
	HasGeniusThink                      bool     `json:"has_genius_think"`
	HasSelfAugmentSkill                 bool     `json:"has_self_augment_skill"`
	HasSelfVerificationDocs             bool     `json:"has_self_verification_docs"`
	HasSelfVerifyCLI                    bool     `json:"has_self_verify_cli"`
	HasSelfAugmentPlanner               bool     `json:"has_self_augment_planner"`
	HasSelfAugmentStateCapture          bool     `json:"has_self_augment_state_capture"`
	HasSelfAugmentLessonCapture         bool     `json:"has_self_augment_lesson_capture"`
	HasAdapterContractMatrix            bool     `json:"has_adapter_contract_matrix"`
	HasRiskQATier                       bool     `json:"has_risk_qa_tier"`
	HasGoalScoreSummary                 bool     `json:"has_goal_score_summary"`
	HasRepoLocalSandbox                 bool     `json:"has_repo_local_sandbox"`
	HasPerformanceBaseline              bool     `json:"has_performance_baseline"`
	HasSelfAugmentSignalTable           bool     `json:"has_self_augment_signal_table"`
	HasQualityInspectCLI                bool     `json:"has_quality_inspect_cli"`
	HasQualityInspectSignals            bool     `json:"has_quality_inspect_signals"`
	HasMCPResourceCoverage              bool     `json:"has_mcp_resource_coverage"`
	HasHostJudgementCoverage            bool     `json:"has_host_judgement_coverage"`
	HasIssueOpsLinkingBoundaryCoverage  bool     `json:"has_issueops_linking_boundary_coverage"`
	HasStateWriteLocking                bool     `json:"has_state_write_locking"`
	HasWorkerStuckRunningDetection      bool     `json:"has_worker_stuck_running_detection"`
	HasIssueOpsInboundAdapterCoverage   bool     `json:"has_issue_ops_inbound_adapter_coverage"`
	HasToolConformanceTransportCoverage bool     `json:"has_tool_conformance_transport_coverage"`
	HasGeniusMermaidLint                bool     `json:"has_genius_mermaid_lint"`
	HasInstallDryRunMode                bool     `json:"has_install_dry_run_mode"`
	HasCLIAdapterSplit                  bool     `json:"has_cli_adapter_split"`
	HasMCPAdapterCatalog                bool     `json:"has_mcp_adapter_catalog"`
	HasCompatibilityContract            bool     `json:"has_compatibility_contract"`
	HasCandidateRefill                  bool     `json:"has_candidate_refill"`
	HasCommandAuditLog                  bool     `json:"has_command_audit_log"`
	HasWorkerMVP                        bool     `json:"has_worker_mvp"`
	HasReleaseReproPack                 bool     `json:"has_release_repro_pack"`
	HasReleaseUserReadme                bool     `json:"has_release_user_readme"`
	HasCrossPlatformBuildMatrix         bool     `json:"has_cross_platform_build_matrix"`
	HasDistributionDecision             bool     `json:"has_distribution_decision"`
	HasReleaseDogfoodNotes              bool     `json:"has_release_dogfood_notes"`
}

const (
	CandidateStatusOpen      = "open"
	CandidateStatusSatisfied = "already_satisfied"
)

// domain/selfaugment는 contract/selfaugment만 import할 수 있어서 qualitycatalog의
// 검증 종류를 여기서 이 이름으로 둔다.
const (
	VerificationToolSignal  = qualitycatalog.ToolSignalKind
	VerificationDocArtifact = qualitycatalog.DocArtifactKind
)
