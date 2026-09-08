// IssueOps CLI가 읽는 구현 검토 DTO다.
package issueops

// IssueOpsImplementationReviewRequest는 구현 diff에 대한 design-review 리뷰 기록이다.
type IssueOpsImplementationReviewRequest struct {
	Verdict        string
	Findings       []string
	Evidence       []string
	ReviewerHost   string
	ReviewerModel  string
	ReviewerEffort string
}

// IssueOpsProjectDocsReviewRequest는 publication 직전 project-doc 반영 판정이다.
type IssueOpsProjectDocsReviewRequest struct {
	Verdict      string
	Docs         []string
	ReviewedDocs []string
	Evidence     []string
}

// IssueOpsSchemaEvidenceRequest는 스키마 변경 사이클의 실측 근거 기록이다.
type IssueOpsSchemaEvidenceRequest struct {
	Measurements    []string
	Sources         []string
	Waive           bool
	WaiverRationale string
}

// 설계 검토 증거 예시 문구는 CLI 도움말과 어댑터가 함께 쓰는 어휘다.
const IssueOpsDesignReviewEvidenceExample = "design review checked alternatives and risks"

// IssueOpsReviewMetricsCycle는 사이클 하나의 적대 리뷰 지표 파생값이다. 새
// durable 상태가 아니라 record가 이미 들고 있는 원자료(devil's-advocate
// history, regress 이벤트, phase ledger)의 읽기 전용 projection이다.
type IssueOpsReviewMetricsCycle struct {
	ID                          string             `json:"id"`
	Phase                       string             `json:"phase,omitempty"`
	DevilsAdvocateRounds        int                `json:"devils_advocate_rounds"`
	DevilsAdvocateVerdicts      []string           `json:"devils_advocate_verdicts,omitempty"`
	RegressCount                int                `json:"regress_count"`
	ImplementationReviewVerdict string             `json:"implementation_review_verdict,omitempty"`
	StageDurationsSeconds       map[string]float64 `json:"stage_durations_seconds,omitempty"`
	ReviewRoundGapsSeconds      []float64          `json:"review_round_gaps_seconds,omitempty"`
}

// IssueOpsReviewMetricsAggregate는 여러 사이클을 가로지르는 요약이다. 평균
// 라운드 수는 리뷰가 있는 사이클만으로 계산한다 — 리뷰가 없는 사이클의 0은
// "라운드가 0회"가 아니라 "측정 대상이 아님"이기 때문이다.
type IssueOpsReviewMetricsAggregate struct {
	Cycles         int     `json:"cycles"`
	ReviewedCycles int     `json:"reviewed_cycles"`
	MeanRounds     float64 `json:"mean_rounds"`
	ReviseRatio    float64 `json:"revise_ratio"`
	StopRatio      float64 `json:"stop_ratio"`
}

// IssueOpsReviewMetricsResult는 `issueops review-metrics`의 응답이다.
type IssueOpsReviewMetricsResult struct {
	OK          bool                           `json:"ok"`
	Cycles      []IssueOpsReviewMetricsCycle   `json:"cycles"`
	Aggregate   IssueOpsReviewMetricsAggregate `json:"aggregate"`
	Warnings    []string                       `json:"warnings,omitempty"`
	GeneratedAt string                         `json:"generated_at"`
}
