// Package issueopsview 패키지는 `issueops view`가 돌려주는 cycle 조망의 DTO를
// 소유한다. 지금은 timeline 조망 하나이며, live record나 cleanup이 남긴 보존
// 요약 중 어느 쪽에서 읽었는지를 결과에 남긴다. contract 계층 패키지다.
package issueopsview

import (
	issueopscontract "issueops/internal/contract/issueops"
	issueopsnextcontract "issueops/internal/contract/issueopsnext"
)

// Record 타입은 공용 IssueOps 기록의 별칭이다. domain/issueopsview는 이 contract
// 패키지만 import할 수 있어서 그 domain이 읽는 공용 어휘를 여기서 이 이름으로 둔다.
type Record = issueopscontract.Record

// CycleOutcome 타입은 cleanup이 record를 지울 때 남기는 보존 요약의 별칭이다.
type CycleOutcome = issueopscontract.CycleOutcome

// CycleTimeline 타입은 cycle 한 건의 시간순 이벤트 목록의 별칭이다.
type CycleTimeline = issueopscontract.CycleTimeline

// TimelineEvent 타입은 시간순 이벤트 한 건의 별칭이다.
type TimelineEvent = issueopscontract.TimelineEvent

// NextResult 타입은 `issueops next`의 결과 별칭이다. 그래프의 stale 표시는 이
// 결과의 missing에서 온다.
type NextResult = issueopsnextcontract.Result

// TimelineLimitRetainedWithoutTimeline names the gap of a retained outcome
// written before timelines existed.
const TimelineLimitRetainedWithoutTimeline = issueopscontract.TimelineLimitRetainedWithoutTimeline

// The views `issueops view` can render.
const (
	// ViewTimeline is the dated event list of one cycle.
	ViewTimeline = "timeline"
	// ViewGraph is the evidence nodes of one cycle and the hash-bound links
	// between them, each marked with what `issueops next` reported.
	ViewGraph = "graph"
)

// The places a Result's data came from.
const (
	// SourceRecord means the live cycle record was read.
	SourceRecord = "record"
	// SourceRetained means the record was gone and the retained cleanup
	// outcome was read instead.
	SourceRetained = "retained"
)

// Result 타입은 `issueops view`의 응답이다. Source가 retained이면 record는
// 이미 지워졌고 Outcome과 RetainedAt이 보존 행의 값을 말한다.
type Result struct {
	OK          bool           `json:"ok"`
	View        string         `json:"view"`
	ID          string         `json:"id"`
	Source      string         `json:"source"`
	GeneratedAt string         `json:"generated_at"`
	Outcome     string         `json:"outcome,omitempty"`
	RetainedAt  string         `json:"retained_at,omitempty"`
	Timeline    *CycleTimeline `json:"timeline,omitempty"`
	Graph       *Graph         `json:"graph,omitempty"`
	Warnings    []string       `json:"warnings,omitempty"`
}

// The state a binding edge can carry. A structural edge has no state.
const (
	// EdgeStateStale means `issueops next` reported the edge's gate as stale.
	EdgeStateStale = "stale"
	// EdgeStateNotReported means `next` did not report the gate. It does not
	// mean the link is fresh: `next` names only the first blocking gate.
	EdgeStateNotReported = "not_reported"
	// EdgeStateUnknown means no judgment could be made; Graph.UnknownReason
	// says why.
	EdgeStateUnknown = "unknown"
)

// The reasons a graph has no judgment for its binding edges.
const (
	// UnknownNextFailed means the `next` call returned an error.
	UnknownNextFailed = "next_failed"
	// UnknownStageUnjudgeable means `next` stopped at a stage that returns
	// before it evaluates the architecture, plan, or implementation gates.
	UnknownStageUnjudgeable = "stage_unjudgeable"
	// UnknownWorktreeMissing means the execution worktree is gone, so the
	// change-set bindings could not be observed.
	UnknownWorktreeMissing = "worktree_missing"
)

// What a binding edge is hashed over.
const (
	// BindingDigest is a document digest.
	BindingDigest = "digest"
	// BindingFingerprint is the change-set content fingerprint.
	BindingFingerprint = "fingerprint"
	// BindingHead is the sealed pull request head.
	BindingHead = "head"
)

const (
	// LimitFirstBlockingGateOnly says `next` reports one blocking gate, so an
	// edge not reported as stale is "not reported", not "fresh".
	LimitFirstBlockingGateOnly = "next_reports_first_blocking_gate_only"
	// LimitDocumentNodesFromReviewRecords says the architecture_doc and plan
	// nodes are read from the review records and the plan path. A document that
	// is staged but not yet reviewed leaves no trace in the record, so its node
	// can be absent.
	LimitDocumentNodesFromReviewRecords = "document_nodes_from_review_records"
	// MaxGraphChildren caps the child nodes a graph lists.
	MaxGraphChildren = 50
)

// Graph 타입은 cycle 한 건의 증거 노드와 연결이다. Phase는 record의 phase이고,
// JudgedBy는 stale 표시가 어느 `next` 판정에서 왔는지를 말한다.
type Graph struct {
	Phase         string      `json:"phase"`
	JudgedBy      JudgedBy    `json:"judged_by"`
	UnknownReason string      `json:"unknown_reason,omitempty"`
	Limits        []string    `json:"limits"`
	Nodes         []GraphNode `json:"nodes"`
	Edges         []GraphEdge `json:"edges"`
	ChildrenTotal int         `json:"children_total"`
	ChildrenShown int         `json:"children_shown"`
}

// JudgedBy 타입은 그래프가 기댄 `next` 판정이다. StaleKeys는 `next`가 보고한
// `_stale` 키 전부이고, UnmappedStaleKeys는 그중 그려진 edge에 매이지 않은 키다.
type JudgedBy struct {
	Stage             string   `json:"stage"`
	StaleKeys         []string `json:"stale_keys"`
	UnmappedStaleKeys []string `json:"unmapped_stale_keys"`
}

// GraphNode 타입은 증거 노드 한 개다. Present가 false이면 아직 없는 증거다.
type GraphNode struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Present bool   `json:"present"`
	At      string `json:"at,omitempty"`
	Verdict string `json:"verdict,omitempty"`
	URL     string `json:"url,omitempty"`
}

// GraphEdge 타입은 노드 사이의 연결이다. Binding, Gate, State는 해시로 묶인
// 연결에만 있다.
type GraphEdge struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Binding string `json:"binding,omitempty"`
	Gate    string `json:"gate,omitempty"`
	State   string `json:"state,omitempty"`
}
