// Package selfverify holds the wire contract of the self-verification loop:
// step results, progress events, goal scores, coverage claims, failure
// clusters, improvement candidates, and LLM evaluation options.
package selfverify

// SelfVerificationContract identifies the versioned self-verification output
// contract: its name, version, stable hash, required fields, goal names, and
// coverage claims.
type SelfVerificationContract struct {
	Name           string   `json:"name"`
	Version        int      `json:"version"`
	Hash           string   `json:"hash"`
	RequiredFields []string `json:"required_fields"`
	GoalNames      []string `json:"goal_names"`
	CoverageClaims []string `json:"coverage_claims"`
}

// SelfVerificationGoalScore is the score of one goal: the percentage of its
// labeled checks that passed, compared with TargetScore.
type SelfVerificationGoalScore struct {
	Name           string   `json:"name"`
	KoreanName     string   `json:"korean_name"`
	Score          float64  `json:"score"`
	TargetScore    float64  `json:"target_score"`
	Passed         bool     `json:"passed"`
	EvidenceLabels []string `json:"evidence_labels"`
	PassedChecks   int      `json:"passed_checks"`
	TotalChecks    int      `json:"total_checks"`
	// NotRun is true when none of the goal's checks executed (for example
	// after fail-fast stopped the pass). Score 0 / Passed false then mean
	// "no evidence", not "evidence failed".
	NotRun bool `json:"not_run"`
}

// SelfVerificationCoverage reports whether the evidence labels backing one
// coverage claim all executed and passed, and which labels are missing.
type SelfVerificationCoverage struct {
	Claim          string   `json:"claim"`
	EvidenceLabels []string `json:"evidence_labels"`
	Covered        bool     `json:"covered"`
	MissingLabels  []string `json:"missing_labels"`
	// NotRun is true when none of the claim's evidence labels executed.
	NotRun bool `json:"not_run"`
}

// SelfVerificationFailureCluster groups failures of one step across iterations,
// listing the seeds that failed it and how many there were.
type SelfVerificationFailureCluster struct {
	Step  string  `json:"step"`
	Seeds []int64 `json:"seeds"`
	Count int     `json:"count"`
}

// SelfVerificationGoalDefinition defines a goal by name, Korean display name,
// and the step labels whose results score it.
type SelfVerificationGoalDefinition struct {
	Name       string
	KoreanName string
	Labels     []string
}

// SelfVerificationCoverageDefinition defines a coverage claim and the step
// labels that must pass to support it.
type SelfVerificationCoverageDefinition struct {
	Claim  string
	Labels []string
}
