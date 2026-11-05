package selfverify

// SelfVerificationCandidate is one improvement candidate of the
// self-verification curriculum, ranked by Priority and Score, with the reasons
// to do it now, the commands that verify it, and, when already satisfied, the
// evidence for that.
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

// CandidateStatus values report whether a candidate still needs work
// (open) or is already satisfied by existing evidence.
const (
	CandidateStatusOpen      = "open"
	CandidateStatusSatisfied = "already_satisfied"
)
