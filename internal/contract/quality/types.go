// Package quality defines the result DTOs of the repository quality inspection: signals, findings, coverage, SNR and status constants.
package quality

import "issueops/internal/contract/qualitycatalog"

// InspectResult is the full quality inspection report with collection, health and gate statuses, signals, findings and improvement candidates.
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

// Summary holds the aggregate counts of open candidates, low-coverage packages, branch-heavy functions and audit items behind an InspectResult.
type Summary struct {
	SelfAugmentOpenCandidates int `json:"self_augment_open_candidates"`
	SelfVerifyOpenCandidates  int `json:"self_verify_open_candidates"`
	LowCoveragePackages       int `json:"low_coverage_packages"`
	BranchCandidateFunctions  int `json:"branch_candidate_functions"`
	HighBranchFunctions       int `json:"high_branch_functions"`
	AuditP1P2Items            int `json:"audit_p1_p2_items"`
	CandidateCount            int `json:"candidate_count"`
}

// Signal is one measured quality metric. Value is compared against Threshold (omitted when the signal has none) to derive Status, and Evidence lists the supporting items.
type Signal struct {
	ID        string   `json:"id"`
	Category  string   `json:"category"`
	Status    string   `json:"status"`
	Value     float64  `json:"value"`
	Threshold float64  `json:"threshold,omitempty"`
	Evidence  []string `json:"evidence"`
}

// CoveragePackage is the statement coverage of one Go package; Coverage is a percentage.
type CoveragePackage struct {
	Package  string  `json:"package"`
	Coverage float64 `json:"coverage"`
}

// BranchFunction reports a function's location and its branch count, used to flag complexity candidates.
type BranchFunction struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Name     string `json:"name"`
	Branches int    `json:"branches"`
}

// AuditItem is one entry of the project audit, with its area, priority and size.
type AuditItem struct {
	ID       string `json:"id"`
	Area     string `json:"area"`
	Title    string `json:"title"`
	Priority string `json:"priority"`
	Size     string `json:"size"`
}

// Finding is a quality problem with its severity, whether it blocks the gate, supporting evidence, a remediation hint and the command that verifies the fix.
type Finding struct {
	ID            string   `json:"id"`
	Severity      string   `json:"severity"`
	Title         string   `json:"title"`
	Blocking      bool     `json:"blocking"`
	Evidence      []string `json:"evidence"`
	Remediation   string   `json:"remediation"`
	VerifyCommand string   `json:"verify_command"`
}

// PioneerCoverage counts the benchmark, reproduction, isolated-evaluation and hidden-holdout results observed against the expected number of pioneer skills, and lists the missing or blocked ones.
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

// PioneerBlockedCase names a skill and evaluation axis whose isolated evaluation was blocked, with the reason.
type PioneerBlockedCase struct {
	Skill  string `json:"skill"`
	Axis   string `json:"axis"`
	Reason string `json:"reason"`
}

// SNRResult is the code signal-to-noise measurement: signal lines (code), noise lines (blank, comment or structural-only), their total, and Ratio as signal over total.
type SNRResult struct {
	SignalLines int     `json:"signal_lines"`
	NoiseLines  int     `json:"noise_lines"`
	TotalLines  int     `json:"total_lines"`
	Ratio       float64 `json:"ratio"`
}

// CollectionStatusOK through GateStatusBlock are the string values of InspectResult's CollectionStatus, HealthStatus and GateStatus fields.
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
