package issueopsview

import (
	"slices"
	"strings"
	"time"

	issueopsviewcontract "issueops/internal/contract/issueopsview"
)

// Judgment is what `issueops next` said about a cycle, reduced to the parts the
// graph reads. Stage and Missing are next's own fields; NextFailed and
// WorktreeMissing are observations the caller made around the call.
type Judgment struct {
	Stage           string
	Missing         []string
	NextFailed      bool
	WorktreeMissing bool
}

// graphEdgeRule is one row of the edge table. A rule with a Binding is a
// hash-bound link and Gate is the `next` key that reports it stale.
type graphEdgeRule struct {
	From, To, Binding, Gate string
}

// graphEdgeRules is the only edge table. An edge is emitted when both of its
// nodes are present. The gate keys are the ones `next` reports in `missing`.
// The cycle -> child:<id> edges depend on the child list, so EvidenceGraph adds
// them after the table rows.
var graphEdgeRules = []graphEdgeRule{
	{From: "intent", To: "architecture_doc"},
	{From: "architecture_doc", To: "architecture_review", Binding: issueopsviewcontract.BindingDigest, Gate: "architecture_review_stale"},
	{From: "architecture_review", To: "plan"},
	{From: "plan", To: "plan_review", Binding: issueopsviewcontract.BindingDigest, Gate: "devils_advocate_review_stale"},
	{From: "plan_review", To: "change_set"},
	{From: "change_set", To: "implementation_review", Binding: issueopsviewcontract.BindingFingerprint, Gate: "implementation_review_stale"},
	{From: "change_set", To: "ai_slop_clean", Binding: issueopsviewcontract.BindingFingerprint, Gate: "ai_slop_clean_stale"},
	{From: "change_set", To: "project_docs_review", Binding: issueopsviewcontract.BindingFingerprint, Gate: "project_docs_review_stale"},
	{From: "change_set", To: "schema_evidence", Binding: issueopsviewcontract.BindingFingerprint, Gate: "schema_evidence_stale"},
	{From: "change_set", To: "pull_request", Binding: issueopsviewcontract.BindingHead, Gate: "pr_head_stale"},
	{From: "implementation_review", To: "pull_request"},
	{From: "pull_request", To: "completion"},
	{From: "parent", To: "cycle"},
}

// judgeableStages are the `next` stages that return only after the
// architecture, plan, and implementation gates were evaluated, so their
// `missing` speaks for the evidence links. Any other stage, a stage added to
// `next` later or an empty key included, is unjudgeable: the list fails closed.
var judgeableStages = []string{
	"plan.write", "plan.architecture", "plan.review", "plan.handoff",
	"implement.enter", "implement", "clean", "docs", "verify",
	"commit-push", "pr.create", "pr.complete",
}

// mergeInProgressKey is the `missing` key of a stage that is held back by an
// unfinished git merge, whatever stage name accompanies it.
const mergeInProgressKey = "merge_in_progress"

// EvidenceGraph builds the evidence graph of record. It never recomputes a
// staleness: a binding edge is stale when its gate key is in judgment.Missing,
// which is the `next` result of the same cycle.
func EvidenceGraph(record issueopsviewcontract.Record, judgment Judgment) issueopsviewcontract.Graph {
	nodes := graphNodes(record)
	present := map[string]bool{}
	for _, node := range nodes {
		present[node.ID] = node.Present
	}

	edges := []issueopsviewcontract.GraphEdge{}
	emitted := map[string]bool{}
	for _, rule := range graphEdgeRules {
		if !present[rule.From] || !present[rule.To] {
			continue
		}
		edge := issueopsviewcontract.GraphEdge{From: rule.From, To: rule.To, Binding: rule.Binding, Gate: rule.Gate}
		if rule.Binding != "" {
			edge.State = edgeState(rule, judgment)
			emitted[rule.Gate] = true
		}
		edges = append(edges, edge)
	}

	children := record.ChildCycles[:min(len(record.ChildCycles), issueopsviewcontract.MaxGraphChildren)]
	for _, child := range children {
		id := "child:" + child.CycleID
		nodes = append(nodes, issueopsviewcontract.GraphNode{
			ID: id, Kind: "child", Present: true, Verdict: child.ValidationVerdict, URL: child.ChildIssueURL,
		})
		edges = append(edges, issueopsviewcontract.GraphEdge{From: "cycle", To: id})
	}

	staleKeys := []string{}
	for _, key := range judgment.Missing {
		if strings.HasSuffix(key, "_stale") {
			staleKeys = append(staleKeys, key)
		}
	}
	slices.Sort(staleKeys)
	staleKeys = slices.Compact(staleKeys)
	unmapped := []string{}
	for _, key := range staleKeys {
		if !emitted[key] {
			unmapped = append(unmapped, key)
		}
	}

	return issueopsviewcontract.Graph{
		Phase:         string(record.Phase),
		JudgedBy:      issueopsviewcontract.JudgedBy{Stage: judgment.Stage, StaleKeys: staleKeys, UnmappedStaleKeys: unmapped},
		UnknownReason: unknownReason(judgment),
		Limits:        []string{issueopsviewcontract.LimitFirstBlockingGateOnly, issueopsviewcontract.LimitDocumentNodesFromReviewRecords},
		Nodes:         nodes,
		Edges:         edges,
		ChildrenTotal: len(record.ChildCycles),
		ChildrenShown: len(children),
	}
}

// stageUnjudgeable reports whether judgment.Missing cannot speak for the
// evidence links: the stage is not a judgeable one, or an unfinished git merge
// holds the stage back.
func stageUnjudgeable(judgment Judgment) bool {
	return !slices.Contains(judgeableStages, judgment.Stage) || slices.Contains(judgment.Missing, mergeInProgressKey)
}

// unknownReason answers why some binding edge may be unjudgeable, or "" when
// none is. The order is fixed: a failed `next` outranks an unjudgeable stage,
// which outranks a missing worktree.
func unknownReason(judgment Judgment) string {
	switch {
	case judgment.NextFailed:
		return issueopsviewcontract.UnknownNextFailed
	case stageUnjudgeable(judgment):
		return issueopsviewcontract.UnknownStageUnjudgeable
	case judgment.WorktreeMissing:
		return issueopsviewcontract.UnknownWorktreeMissing
	}
	return ""
}

// edgeState judges one binding edge. A gate `next` reported stale is stale
// whatever else is unknown. Otherwise a failed `next` or an unjudgeable stage
// leaves the edge unknown, and a missing worktree does so only for the edges
// hashed over the worktree: a document digest does not need it.
func edgeState(rule graphEdgeRule, judgment Judgment) string {
	switch {
	case slices.Contains(judgment.Missing, rule.Gate):
		return issueopsviewcontract.EdgeStateStale
	case judgment.NextFailed || stageUnjudgeable(judgment):
		return issueopsviewcontract.EdgeStateUnknown
	case judgment.WorktreeMissing && rule.Binding != issueopsviewcontract.BindingDigest:
		return issueopsviewcontract.EdgeStateUnknown
	}
	return issueopsviewcontract.EdgeStateNotReported
}

// architectureDigestRecorded reports that a review was bound to an architecture
// document: the current or an earlier round recorded its digest. Approval is
// the architecture_review node's business, not this one.
func architectureDigestRecorded(record issueopsviewcontract.Record) bool {
	review := record.ArchitectureReview
	if review == nil {
		return false
	}
	if review.ReviewedArchitectureDigest != "" {
		return true
	}
	for _, round := range review.History {
		if round.ReviewedArchitectureDigest != "" {
			return true
		}
	}
	return false
}

// planDigestRecorded reports that a plan review was bound to a plan: the
// current or an earlier round recorded its digest.
func planDigestRecorded(record issueopsviewcontract.Record) bool {
	review := record.DevilsAdvocateReview
	if review == nil {
		return false
	}
	if review.ReviewedPlanDigest != "" {
		return true
	}
	for _, round := range review.History {
		if round.ReviewedPlanDigest != "" {
			return true
		}
	}
	return false
}

// graphNodes lists every fixed node in display order, present or not. Child
// nodes are appended by the caller.
func graphNodes(record issueopsviewcontract.Record) []issueopsviewcontract.GraphNode {
	node := func(id string, present bool) issueopsviewcontract.GraphNode {
		return issueopsviewcontract.GraphNode{ID: id, Kind: id, Present: present}
	}
	nodes := []issueopsviewcontract.GraphNode{node("cycle", true)}

	intent := node("intent", record.Intent != nil)
	if record.Intent != nil {
		intent.At = record.Intent.RecordedAt
	}
	nodes = append(nodes, intent, node("architecture_doc", architectureDigestRecorded(record)))

	architecture := node("architecture_review", record.ArchitectureReview != nil && record.ArchitectureReview.Approved)
	if architecture.Present {
		architecture.At = record.ArchitectureReview.ReviewedAt
	}
	nodes = append(nodes, architecture, node("plan", record.PlanPath != "" || planDigestRecorded(record)))

	planReview := node("plan_review", record.DevilsAdvocateReview != nil)
	if review := record.DevilsAdvocateReview; review != nil {
		planReview.Verdict, planReview.At = review.Verdict, review.RecordedAt
	}
	nodes = append(nodes, planReview, node("change_set", record.Execution != nil))

	implementation := node("implementation_review", record.ImplementationReview != nil)
	if review := record.ImplementationReview; review != nil {
		implementation.Verdict, implementation.At = review.Verdict, review.RecordedAt
	}
	slop := node("ai_slop_clean", record.AISlopCleanAt != "")
	slop.At = record.AISlopCleanAt
	nodes = append(nodes, implementation, slop)

	docs := node("project_docs_review", record.ProjectDocsReview != nil)
	if review := record.ProjectDocsReview; review != nil {
		docs.Verdict, docs.At = review.Verdict, review.RecordedAt
	}
	schema := node("schema_evidence", record.SchemaEvidence != nil)
	if evidence := record.SchemaEvidence; evidence != nil {
		schema.At = evidence.RecordedAt
	}
	nodes = append(nodes, docs, schema)

	pull := node("pull_request", record.RemoteArtifact != nil)
	if artifact := record.RemoteArtifact; artifact != nil {
		pull.URL, pull.At = artifact.URL, artifact.VerifiedAt
	}
	completion := node("completion", record.Execution != nil && record.Execution.Completion != nil)
	if completion.Present {
		completion.At = record.Execution.Completion.CompletedAt
	}
	nodes = append(nodes, pull, completion, node("parent", record.Delegation != nil))
	for i := range nodes {
		nodes[i].At = graphNodeAt(nodes[i].At)
	}
	return nodes
}

// graphNodeAt formats raw as UTC time.RFC3339Nano, the form the timeline and
// the inventory use, or returns "" when raw is empty or does not parse.
func graphNodeAt(raw string) string {
	at, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return ""
	}
	return at.UTC().Format(time.RFC3339Nano)
}
