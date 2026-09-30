package selfverify

type SelfVerificationContract struct {
	Name           string   `json:"name"`
	Version        int      `json:"version"`
	Hash           string   `json:"hash"`
	RequiredFields []string `json:"required_fields"`
	GoalNames      []string `json:"goal_names"`
	CoverageClaims []string `json:"coverage_claims"`
}

type SelfVerificationGoalScore struct {
	Name           string   `json:"name"`
	KoreanName     string   `json:"korean_name"`
	Score          float64  `json:"score"`
	TargetScore    float64  `json:"target_score"`
	Passed         bool     `json:"passed"`
	EvidenceLabels []string `json:"evidence_labels"`
	PassedChecks   int      `json:"passed_checks"`
	TotalChecks    int      `json:"total_checks"`
}

type SelfVerificationCoverage struct {
	Claim          string   `json:"claim"`
	EvidenceLabels []string `json:"evidence_labels"`
	Covered        bool     `json:"covered"`
	MissingLabels  []string `json:"missing_labels"`
}

type SelfVerificationFailureCluster struct {
	Step  string  `json:"step"`
	Seeds []int64 `json:"seeds"`
	Count int     `json:"count"`
}

type SelfVerificationGoalDefinition struct {
	Name       string
	KoreanName string
	Labels     []string
}

type SelfVerificationCoverageDefinition struct {
	Claim  string
	Labels []string
}
