package issueopsreview

// DevilsAdvocateReview is the versioned review payload stored on a cycle.
type DevilsAdvocateReview struct {
	Verdict            string                `json:"verdict"`
	Findings           []string              `json:"findings,omitempty"`
	Waived             bool                  `json:"waived,omitempty"`
	WaiverRationale    string                `json:"waiver_rationale,omitempty"`
	ReviewerPattern    string                `json:"reviewer_pattern,omitempty"`
	ReviewerContext    string                `json:"reviewer_context,omitempty"`
	ReviewedPlanDigest string                `json:"reviewed_plan_digest,omitempty"`
	History            []DevilsAdvocateRound `json:"history,omitempty"`
	RecordedAt         string                `json:"recorded_at"`
	IssueReflectedAt   string                `json:"issue_reflected_at,omitempty"`
}

type DevilsAdvocateRound struct {
	Verdict            string   `json:"verdict"`
	Findings           []string `json:"findings,omitempty"`
	Waived             bool     `json:"waived,omitempty"`
	WaiverRationale    string   `json:"waiver_rationale,omitempty"`
	ReviewerContext    string   `json:"reviewer_context,omitempty"`
	ReviewedPlanDigest string   `json:"reviewed_plan_digest,omitempty"`
	RecordedAt         string   `json:"recorded_at"`
}

type DevilsAdvocateReviewRequest struct {
	Verdict         string
	Findings        []string
	Waived          bool
	WaiverRationale string
	ReviewerContext string
}

// RegressionPreconditions is the pure snapshot needed before child liveness is read.
type RegressionPreconditions struct {
	CycleID      string
	Phase        string
	Review       *DevilsAdvocateReview
	RegressCount int
}

// RegressionChange describes the record mutation after eligibility and child
// checks; persistence remains with the application boundary.
type RegressionChange struct {
	FromPhase           string
	ToPhase             string
	EventReason         string
	At                  string
	DecisionTitle       string
	DecisionBody        string
	DecisionKind        string
	DecisionRationale   string
	StalePhases         []string
	StaleNote           string
	ClearDesignApproval bool
	ClearReview         bool
}
