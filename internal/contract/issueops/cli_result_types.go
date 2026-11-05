// IssueOps CLI가 읽는 구현 검토 DTO다.

package issueops

// ImplementationReviewRequest 타입은 구현 diff에 대한 architecture-review 리뷰 기록이다.
type ImplementationReviewRequest struct {
	Verdict         string
	Findings        []string
	Evidence        []string
	ReviewerHost    string
	ReviewerModel   string
	ReviewerEffort  string
	ReviewerContext string // subagent | inline, like the plan reviews (audit only)
}

// ProjectDocsReviewRequest 타입은 publication 직전 project-doc 반영 판정이다.
type ProjectDocsReviewRequest struct {
	Verdict      string
	Docs         []string
	ReviewedDocs []string
	Evidence     []string
}

// SchemaEvidenceRequest 타입은 스키마 변경 사이클의 실측 근거 기록이다.
type SchemaEvidenceRequest struct {
	Measurements    []string
	Sources         []string
	Waive           bool
	WaiverRationale string
}

// MutationAck is the --json answer of a record mutation. It names the
// record and its position instead of echoing the whole record (which grew to
// 25+ KB per call over a cycle); `issueops status --id ID --json` returns the
// full record when a caller needs it.
type MutationAck struct {
	OK            bool   `json:"ok"`
	SchemaVersion int    `json:"schema_version"`
	ID            string `json:"id"`
	Phase         Phase  `json:"phase"`
	Branch        string `json:"branch,omitempty"`
	WorktreePath  string `json:"worktree_path,omitempty"`
	UpdatedAt     string `json:"updated_at"`
}

// NewMutationAck projects record onto its mutation acknowledgement.
func NewMutationAck(record Record) MutationAck {
	return MutationAck{
		OK: record.OK, SchemaVersion: record.SchemaVersion, ID: record.ID, Phase: record.Phase,
		Branch: record.Branch, WorktreePath: record.WorktreePath, UpdatedAt: record.UpdatedAt,
	}
}

// BranchPrepareAck is the `branch prepare --json` answer: the mutation
// acknowledgement plus the branch-creation steps the caller acts on next.
type BranchPrepareAck struct {
	MutationAck
	Steps []BranchPrepareStep `json:"steps,omitempty"`
}

// NewBranchPrepareAck projects record onto the branch prepare acknowledgement.
// The link-creation steps are left out once the link is recorded verified:
// creating a link next to "verified" reads as a contradiction.
func NewBranchPrepareAck(record Record) BranchPrepareAck {
	ack := BranchPrepareAck{MutationAck: NewMutationAck(record)}
	if record.BranchPrepare != nil && !record.BranchPrepare.LinkVerified {
		ack.Steps = record.BranchPrepare.Steps
	}
	return ack
}

// StatusView is the record as `status --json` prints it: the stored record
// without the static branch-creation steps (already shown by branch prepare).
func StatusView(record Record) Record {
	if record.BranchPrepare != nil && len(record.BranchPrepare.Steps) > 0 {
		prepare := *record.BranchPrepare
		prepare.Steps = nil
		record.BranchPrepare = &prepare
	}
	return record
}

// ArchitectureReviewEvidenceExample 상수는 아키텍처 검토 증거 예시 문구이며,
// CLI 도움말과 어댑터가 함께 쓰는 어휘다.
const ArchitectureReviewEvidenceExample = "architecture review checked alternatives and risks"

// IDFormatHint follows an id that is not a cycle id: what one looks like and
// where to find it, for callers (an MCP-only agent) that guessed one.
const IDFormatHint = "cycle ids look like io-1a2b3c4d5e6f; find them with `issueops next --json` or `issueops list --repo PATH --json`"

// ReviewMetricsCycle 타입은 사이클 하나의 적대 리뷰 지표 파생값이다. 살아
// 있는 record에서는 그 record가 이미 들고 있는 원자료(devil's-advocate
// history, regress 이벤트, phase ledger)의 읽기 전용 projection이다. Outcome이
// 있으면 cleanup이 record를 지우며 남긴 CycleOutcome에서 읽은 값이다.
type ReviewMetricsCycle struct {
	ID                          string             `json:"id"`
	Phase                       string             `json:"phase,omitempty"`
	Outcome                     string             `json:"outcome,omitempty"`
	DevilsAdvocateRounds        int                `json:"devils_advocate_rounds"`
	DevilsAdvocateVerdicts      []string           `json:"devils_advocate_verdicts,omitempty"`
	RegressCount                int                `json:"regress_count"`
	ImplementationReviewVerdict string             `json:"implementation_review_verdict,omitempty"`
	StageDurationsSeconds       map[string]float64 `json:"stage_durations_seconds,omitempty"`
	ReviewRoundGapsSeconds      []float64          `json:"review_round_gaps_seconds,omitempty"`
}

// CycleOutcomeSchemaVersion 상수는 보존된 사이클 결과 row의 현재 쓰기 버전이다.
const CycleOutcomeSchemaVersion = 1

// cleanup이 record를 지울 때 남기는 사이클 결과 종류.
const (
	CycleOutcomeFinished  = "finished"
	CycleOutcomeAbandoned = "abandoned"
)

// CycleOutcome 타입은 cleanup finish·abandon이 record를 지우는 같은
// 트랜잭션에서 남기는 지표 요약이다. record 원문이 아니라 review-metrics가
// 파생하는 수치와 판정만 담으므로, record가 사라진 뒤에도 하네스 단계별 효과를
// 사이클을 가로질러 비교할 수 있다.
type CycleOutcome struct {
	SchemaVersion int                `json:"schema_version"`
	ID            string             `json:"id"`
	Repo          string             `json:"repo"`
	Outcome       string             `json:"outcome"`
	RetainedAt    string             `json:"retained_at"`
	Metrics       ReviewMetricsCycle `json:"metrics"`
}

// ReviewMetricsAggregate 타입은 여러 사이클을 가로지르는 요약이다. 평균
// 라운드 수는 리뷰가 있는 사이클만으로 계산한다 — 리뷰가 없는 사이클의 0은
// "라운드가 0회"가 아니라 "측정 대상이 아님"이기 때문이다.
type ReviewMetricsAggregate struct {
	Cycles         int     `json:"cycles"`
	ReviewedCycles int     `json:"reviewed_cycles"`
	MeanRounds     float64 `json:"mean_rounds"`
	ReviseRatio    float64 `json:"revise_ratio"`
	StopRatio      float64 `json:"stop_ratio"`
}

// ReviewMetricsResult 타입은 `issueops review-metrics`의 응답이다.
type ReviewMetricsResult struct {
	OK     bool                 `json:"ok"`
	Cycles []ReviewMetricsCycle `json:"cycles"`
	// ReadErrors는 집계에서 빠진 사이클 수다. 모집단이 줄었다는 사실을 숨기면
	// revise 비율이 어떤 사이클들에서 나온 값인지 알 수 없다(issueops list 선례).
	ReadErrors  int                    `json:"read_errors,omitempty"`
	Aggregate   ReviewMetricsAggregate `json:"aggregate"`
	Warnings    []string               `json:"warnings,omitempty"`
	GeneratedAt string                 `json:"generated_at"`
}
