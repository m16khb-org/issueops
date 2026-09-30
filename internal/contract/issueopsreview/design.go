package issueopsreview

type DesignReview struct {
	ProblemSummary string   `json:"problem_summary"`
	ProposedDesign string   `json:"proposed_design"`
	RefactorPlan   string   `json:"refactor_plan,omitempty"`
	Alternatives   []string `json:"alternatives,omitempty"`
	Risks          []string `json:"risks,omitempty"`
	Verification   []string `json:"verification"`
	OpenQuestions  []string `json:"open_questions,omitempty"`
	Approved       bool     `json:"approved"`
	ReviewedAt     string   `json:"reviewed_at"`
}

type DesignReviewRequest struct {
	ProblemSummary string
	ProposedDesign string
	RefactorPlan   string
	Alternatives   []string
	Risks          []string
	Verification   []string
	OpenQuestions  []string
	Approved       bool
}
