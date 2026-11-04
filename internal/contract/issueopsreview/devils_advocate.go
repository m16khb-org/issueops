package issueopsreview

// DevilsAdvocateReview is the versioned review payload stored on a cycle.
type DevilsAdvocateReview struct {
	Verdict            string                `json:"verdict"`
	Findings           []string              `json:"findings,omitempty"`
	Waived             bool                  `json:"waived,omitempty"`
	WaiverRationale    string                `json:"waiver_rationale,omitempty"`
	ReviewerPattern    string                `json:"reviewer_pattern,omitempty"`
	ReviewerContext    string                `json:"reviewer_context,omitempty"`
	ReviewerHost       string                `json:"reviewer_host,omitempty"`
	ReviewerModel      string                `json:"reviewer_model,omitempty"`
	ReviewerEffort     string                `json:"reviewer_effort,omitempty"`
	ReviewedPlanDigest string                `json:"reviewed_plan_digest,omitempty"`
	History            []DevilsAdvocateRound `json:"history,omitempty"`
	RecordedAt         string                `json:"recorded_at"`
	IssueReflectedAt   string                `json:"issue_reflected_at,omitempty"`
}

// DevilsAdvocateRound 타입은 재기록으로 대체된 이전 devil's advocate 검토
// 기록이다.
type DevilsAdvocateRound struct {
	Verdict            string   `json:"verdict"`
	Findings           []string `json:"findings,omitempty"`
	Waived             bool     `json:"waived,omitempty"`
	WaiverRationale    string   `json:"waiver_rationale,omitempty"`
	ReviewerContext    string   `json:"reviewer_context,omitempty"`
	ReviewerHost       string   `json:"reviewer_host,omitempty"`
	ReviewerModel      string   `json:"reviewer_model,omitempty"`
	ReviewerEffort     string   `json:"reviewer_effort,omitempty"`
	ReviewedPlanDigest string   `json:"reviewed_plan_digest,omitempty"`
	RecordedAt         string   `json:"recorded_at"`
}

// DevilsAdvocateReviewRequest 타입은 devil's advocate 검토를 기록하려는
// 호출자가 넘기는 입력이다. Reviewer* 필드는 검토를 실제로 수행한 호스트,
// 모델, 노력 수준을 남기는 감사용 필드다.
type DevilsAdvocateReviewRequest struct {
	Verdict         string
	Findings        []string
	Waived          bool
	WaiverRationale string
	ReviewerContext string
	// Reviewer* are audit fields: the model and effort the review actually
	// ran with (the session's own for an inline review).
	ReviewerHost, ReviewerModel, ReviewerEffort string
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
	FromPhase                 string
	ToPhase                   string
	EventReason               string
	At                        string
	DecisionTitle             string
	DecisionBody              string
	DecisionKind              string
	DecisionRationale         string
	StalePhases               []string
	StaleNote                 string
	ClearArchitectureApproval bool
	ClearReview               bool
}
