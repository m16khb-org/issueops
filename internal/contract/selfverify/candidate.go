package selfverify

type SelfVerificationCandidate struct {
	Priority             int      `json:"priority"`
	ID                   string   `json:"id"`
	Category             string   `json:"category"`
	Status               string   `json:"status"`
	Score                float64  `json:"score"`
	WhyNow               []string `json:"why_now"`
	VerifyWith           []string `json:"verify_with"`
	SatisfactionEvidence []string `json:"satisfaction_evidence,omitempty"`
}

const (
	CandidateStatusOpen      = "open"
	CandidateStatusSatisfied = "already_satisfied"
)
