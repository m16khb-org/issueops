package issueopsreview

// ArchitectureReview는 plan 단계가 승인한 아키텍처 문서의 검토 기록이다.
// ReviewedArchitectureDigest는 승인한 문서의 sha256이고, 문서가 바뀌면
// 승인은 stale이 된다.
type ArchitectureReview struct {
	ProblemSummary             string                    `json:"problem_summary"`
	ProposedArchitecture       string                    `json:"proposed_architecture"`
	RefactorPlan               string                    `json:"refactor_plan,omitempty"`
	Alternatives               []string                  `json:"alternatives,omitempty"`
	Risks                      []string                  `json:"risks,omitempty"`
	Verification               []string                  `json:"verification"`
	OpenQuestions              []string                  `json:"open_questions,omitempty"`
	Approved                   bool                      `json:"approved"`
	ReviewerContext            string                    `json:"reviewer_context,omitempty"`
	ReviewerHost               string                    `json:"reviewer_host,omitempty"`
	ReviewerModel              string                    `json:"reviewer_model,omitempty"`
	ReviewerEffort             string                    `json:"reviewer_effort,omitempty"`
	ReviewedArchitectureDigest string                    `json:"reviewed_architecture_digest,omitempty"`
	History                    []ArchitectureReviewRound `json:"history,omitempty"`
	ReviewedAt                 string                    `json:"reviewed_at"`
}

// ArchitectureReviewRound는 재기록으로 대체된 이전 검토 기록이다.
type ArchitectureReviewRound struct {
	Approved                   bool     `json:"approved"`
	Verification               []string `json:"verification,omitempty"`
	OpenQuestions              []string `json:"open_questions,omitempty"`
	ReviewerContext            string   `json:"reviewer_context,omitempty"`
	ReviewerHost               string   `json:"reviewer_host,omitempty"`
	ReviewerModel              string   `json:"reviewer_model,omitempty"`
	ReviewerEffort             string   `json:"reviewer_effort,omitempty"`
	ReviewedArchitectureDigest string   `json:"reviewed_architecture_digest,omitempty"`
	ReviewedAt                 string   `json:"reviewed_at"`
}

type ArchitectureReviewRequest struct {
	ProblemSummary       string
	ProposedArchitecture string
	RefactorPlan         string
	Alternatives         []string
	Risks                []string
	Verification         []string
	OpenQuestions        []string
	Approved             bool
	ReviewerContext      string
	// Reviewer* are audit fields: the model and effort the review ran with.
	ReviewerHost, ReviewerModel, ReviewerEffort string
}
