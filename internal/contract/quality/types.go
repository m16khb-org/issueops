package quality

import "issueops/internal/contract/qualitycatalog"

type InspectResult struct {
	OK               bool                       `json:"ok"`
	CollectionStatus string                     `json:"collection_status"`
	HealthStatus     string                     `json:"health_status"`
	GateStatus       string                     `json:"gate_status"`
	GeneratedAt      string                     `json:"generated_at"`
	IssueOpsRoot     string                     `json:"issueops_root"`
	Summary          Summary                    `json:"summary"`
	Signals          []Signal                   `json:"signals"`
	Findings         []Finding                  `json:"findings"`
	PioneerCoverage  PioneerCoverage            `json:"pioneer_coverage"`
	Candidates       []qualitycatalog.Candidate `json:"candidates"`
	Warnings         []string                   `json:"warnings"`
}

type Summary struct {
	SelfAugmentOpenCandidates int `json:"self_augment_open_candidates"`
	SelfVerifyOpenCandidates  int `json:"self_verify_open_candidates"`
	LowCoveragePackages       int `json:"low_coverage_packages"`
	BranchCandidateFunctions  int `json:"branch_candidate_functions"`
	HighBranchFunctions       int `json:"high_branch_functions"`
	AuditP1P2Items            int `json:"audit_p1_p2_items"`
	CandidateCount            int `json:"candidate_count"`
}

type Signal struct {
	ID        string   `json:"id"`
	Category  string   `json:"category"`
	Status    string   `json:"status"`
	Value     float64  `json:"value"`
	Threshold float64  `json:"threshold,omitempty"`
	Evidence  []string `json:"evidence"`
}

type CoveragePackage struct {
	Package  string  `json:"package"`
	Coverage float64 `json:"coverage"`
}

type BranchFunction struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Name     string `json:"name"`
	Branches int    `json:"branches"`
}

type AuditItem struct {
	ID       string `json:"id"`
	Area     string `json:"area"`
	Title    string `json:"title"`
	Priority string `json:"priority"`
	Size     string `json:"size"`
}

type Finding struct {
	ID            string   `json:"id"`
	Severity      string   `json:"severity"`
	Title         string   `json:"title"`
	Blocking      bool     `json:"blocking"`
	Evidence      []string `json:"evidence"`
	Remediation   string   `json:"remediation"`
	VerifyCommand string   `json:"verify_command"`
}

type PioneerCoverage struct {
	Expected               int                  `json:"expected"`
	BenchmarkObserved      int                  `json:"benchmark_observed"`
	BenchmarkMissing       []string             `json:"benchmark_missing"`
	ReproductionObserved   int                  `json:"reproduction_observed"`
	ReproductionMissing    []string             `json:"reproduction_missing"`
	IsolatedExpected       int                  `json:"isolated_expected"`
	IsolatedObserved       int                  `json:"isolated_observed"`
	IsolatedPassed         int                  `json:"isolated_passed"`
	IsolatedBlocked        int                  `json:"isolated_blocked"`
	IsolatedFailed         int                  `json:"isolated_failed"`
	IsolatedExecutionCount int                  `json:"isolated_execution_count"`
	IsolatedBlockedCases   []PioneerBlockedCase `json:"isolated_blocked_cases"`
	HiddenHoldoutObserved  int                  `json:"hidden_holdout_observed"`
}

type PioneerBlockedCase struct {
	Skill  string `json:"skill"`
	Axis   string `json:"axis"`
	Reason string `json:"reason"`
}

type SNRResult struct {
	SignalLines int     `json:"signal_lines"`
	NoiseLines  int     `json:"noise_lines"`
	TotalLines  int     `json:"total_lines"`
	Ratio       float64 `json:"ratio"`
}

const (
	CollectionStatusOK    = "ok"
	CollectionStatusError = "error"

	HealthStatusHealthy        = "healthy"
	HealthStatusNeedsAttention = "needs_attention"
	HealthStatusUnknown        = "unknown"

	GateStatusPass       = "pass"
	GateStatusReportOnly = "report_only"
	GateStatusBlock      = "block"
)
