// Package issueopsbenchmark 패키지는 benchmark 실행의 요청·결과·집계 DTO를
// 정의하는 계약(contract) 계층이다. 값을 만들어내는 쪽은 I/O를 하지만
// 읽고 전달하는 쪽은 그 구현을 알 필요가 없다.
package issueopsbenchmark

// IssueOpsDimensionScore 타입은 평가 차원(dimension) 하나의 점수와 그 근거다.
// NotApplicable이면 해당 fixture에 적용되지 않는 차원이다.
type IssueOpsDimensionScore struct {
	Dimension     string  `json:"dimension"`
	Score         float64 `json:"score"`
	Evidence      string  `json:"evidence"`
	NotApplicable bool    `json:"not_applicable,omitempty"`
}

// Score 타입은 fixture 하나의 평가 결과다. 차원별 점수와 평균·최소 점수,
// 결정적 검사·judge·치명적 실패 목록, 통과 여부를 담는다.
type Score struct {
	OK                    bool                     `json:"ok"`
	FixtureID             string                   `json:"fixture_id"`
	AverageScore          float64                  `json:"average_score"`
	MinimumScore          float64                  `json:"minimum_score"`
	DimensionScores       []IssueOpsDimensionScore `json:"dimension_scores"`
	DeterministicFailures []string                 `json:"deterministic_failures"`
	JudgeFailures         []string                 `json:"judge_failures"`
	CriticalFailures      []string                 `json:"critical_failures"`
	Passed                bool                     `json:"passed"`
}

// RunResult 타입은 benchmark 전체 실행의 집계다. fixture별 Score와 평균·최소
// 점수, 치명적 실패 개수를 담는다.
type RunResult struct {
	OK                   bool    `json:"ok"`
	ID                   string  `json:"id"`
	FixtureCount         int     `json:"fixture_count"`
	AverageScore         float64 `json:"average_score"`
	MinimumScore         float64 `json:"minimum_score"`
	CriticalFailureCount int     `json:"critical_failure_count"`
	Scores               []Score `json:"scores"`
}

// CompareResult 타입은 기준(baseline) run과 후보(candidate) run의 점수 차이다.
// Delta는 후보에서 기준을 뺀 값이고 Regressions는 나빠진 항목 목록이다.
type CompareResult struct {
	OK                   bool     `json:"ok"`
	Improved             bool     `json:"improved"`
	BaselineID           string   `json:"baseline_id"`
	CandidateID          string   `json:"candidate_id"`
	AverageScoreDelta    float64  `json:"average_score_delta"`
	MinimumScoreDelta    float64  `json:"minimum_score_delta"`
	CriticalFailureDelta int      `json:"critical_failure_delta"`
	Regressions          []string `json:"regressions"`
}

// IssueOpsAutoresearchCandidate 타입은 autoresearch 루프가 제안한 개선 후보
// 하나다. 가설, 개선 대상 차원, 수정이 허용되는 파일 범위(EditSurface)를 담는다.
type IssueOpsAutoresearchCandidate struct {
	ID               string   `json:"id"`
	Hypothesis       string   `json:"hypothesis"`
	TargetDimensions []string `json:"target_dimensions"`
	EditSurface      []string `json:"edit_surface"`
}

// IssueOpsAutoresearchGateRequest 타입은 후보를 유지할지 판정하는 gate의
// 입력이다. 후보와 기준·후보 실행 결과, 실제로 바뀐 경로를 담는다.
type IssueOpsAutoresearchGateRequest struct {
	Candidate    IssueOpsAutoresearchCandidate
	BaselineRun  RunResult
	CandidateRun RunResult
	ChangedPaths []string
}

// IssueOpsAutoresearchGateResult 타입은 autoresearch gate의 판정이다.
// 후보 유지 여부와 benchmark 비교, 수정 범위 위반, 대상 차원 회귀, 폐기 사유를 담는다.
type IssueOpsAutoresearchGateResult struct {
	OK                         bool          `json:"ok"`
	KeepCandidate              bool          `json:"keep_candidate"`
	CandidateID                string        `json:"candidate_id"`
	BenchmarkCompare           CompareResult `json:"benchmark_compare"`
	EditSurfaceViolations      []string      `json:"edit_surface_violations,omitempty"`
	TargetDimensionRegressions []string      `json:"target_dimension_regressions,omitempty"`
	DiscardReasons             []string      `json:"discard_reasons,omitempty"`
}

// IssueOpsJudgeMap 타입은 A7 judge 출처(provenance) 래퍼다. --judge 파일
// 백엔드는 외부에서 만든 judge 점수를 새 결정적 실행에 합치는데, 이 타입은 그
// 점수를 judge가 평가한 run의 출처와 함께 담아 점수가 자기가 채점한 run에
// 몰래 귀속되지 않게 한다.
//
// 정직한 범위(issueops_reliability.go의 RecordedRun 출처 guard와 같다):
// ValidateJudgeProvenance는 자기 참조 guard일 뿐 judge 독립성의 증명이 아니다.
// 원천 run이 채점 대상 run 자신이면 거부하고 원천 run이 디스크에 실제로
// 있어야 한다고 요구해, "아무 문자열이나 적기"에서 "실재하는 이전 run 지목하기"로
// 기준을 올릴 뿐이다. 독립 judge가 다른 산출물을 평가했다는 점은 보장하지
// 않으므로 대시보드는 이 gate만으로 "독립 judge"라고 주장해서는 안 된다.
type IssueOpsJudgeMap struct {
	SourceRunID string           `json:"source_run_id"`
	Provenance  string           `json:"provenance"`
	Scores      map[string]Score `json:"scores"`
}

// RecordedRun 타입은 SUT를 offline으로 한 번 실행한 fixture별 pass/fail 결과다.
// RunID와 Provenance는 필수이고 RunID는 run마다 달라야 한다. 이는 기계적으로
// 확인하는 독립성 guard다.
type RecordedRun struct {
	RunID      string          `json:"run_id"`
	Provenance string          `json:"provenance"`
	Outcomes   map[string]bool `json:"outcomes"` // fixtureID -> passed
}

// RecordedOutcomes 타입은 `issueops benchmark reliability`의 offline 입력
// 형식이다. fixture 집합이 정렬된 benchmark 전체 재실행 k회를 담는다.
type RecordedOutcomes struct {
	Runs []RecordedRun `json:"runs"`
}

// FixtureReliability 타입은 fixture별 분석이다. Clopper-Pearson interval을
// 의도적으로 fixture별로 계산한다. 이질적인 fixture를 하나의 (c,n)으로 합치면
// 단일 Bernoulli p를 가정하게 되어 지나치게 좁은 interval을 보고하게 된다.
type FixtureReliability struct {
	FixtureID     string  `json:"fixture_id"`
	Trials        int     `json:"trials"`
	Successes     int     `json:"successes"`
	PassAt1       float64 `json:"pass_at_1"`
	IntervalLow   float64 `json:"interval_low"`
	IntervalHigh  float64 `json:"interval_high"`
	IntervalWidth float64 `json:"interval_width"`
}

// PassPowKPoint 타입은 suite 수준 pass^k 신뢰도 곡선의 한 점이다.
type PassPowKPoint struct {
	K        int     `json:"k"`
	PassPowK float64 `json:"pass_pow_k"`
}

// ReliabilityReport 타입은 k개의 offline 기록 run에서 SUT 신뢰도를 요약한다.
// pass^k는 tau-bench 정의를 따른다. suite 곡선은 fixture별 C(c_i,k)/C(n_i,k)의
// 평균이며, MacroPassAt1은 시행 수가 많은 fixture가 지배하지 않도록 fixture별
// 성공률을 평균한 값이다. deterministic scorer gate에는 쓰지 않는다.
type ReliabilityReport struct {
	Runs          int                  `json:"runs"`
	Alpha         float64              `json:"alpha"`
	MacroPassAt1  float64              `json:"macro_pass_at_1"`
	MaxK          int                  `json:"max_k"` // = min_i trials_i
	PassPowKCurve []PassPowKPoint      `json:"pass_pow_k_curve"`
	Fixtures      []FixtureReliability `json:"fixtures"`
	Provenance    []string             `json:"provenance"`
}

// JudgeSample is one offline-recorded judge verdict of the same artifact. SampleID
// is REQUIRED and must be DISTINCT across samples, and Provenance must be
// non-empty: this is the machine-checkable guard (ported from the RecordedRun /
// judge-provenance guards) against re-using one judge sample as N fake voters.
type JudgeSample struct {
	SampleID   string `json:"sample_id"`
	Provenance string `json:"provenance"`
	Score      Score  `json:"score"`
}

// ConsensusVerdict is the self-consistency aggregation of N judge samples.
type ConsensusVerdict struct {
	Samples int `json:"samples"`
	// MajorityPassed is the majority vote over each sample's BINARIZED pass/fail
	// (the gate's own boolean) — the faithful "majority vote". Ties resolve to
	// false (fail-closed) and surface as PassAgreement 0.5.
	MajorityPassed bool    `json:"majority_passed"`
	PassAgreement  float64 `json:"pass_agreement"`
	// MedianAverageScore is the robust consensus point of the N average scores.
	// Median, NOT mean: the deterministic scorer is bimodal (0/100), so a mean
	// would land on a value no sample produced and the gate can never accept.
	MedianAverageScore float64  `json:"median_average_score"`
	ScoreMin           float64  `json:"score_min"`
	ScoreMax           float64  `json:"score_max"`
	ScoreSpread        float64  `json:"score_spread"`
	SampleVariance     float64  `json:"sample_variance"`
	Provenance         []string `json:"provenance"`
	Caveat             string   `json:"caveat"`
}
