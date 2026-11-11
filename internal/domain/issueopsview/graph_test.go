package issueopsview

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"slices"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsnextcontract "issueops/internal/contract/issueopsnext"
	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsviewcontract "issueops/internal/contract/issueopsview"
)

// fullGraphRecord has every evidence node present, so every edge in the table
// is drawn.
func fullGraphRecord() issueopsviewcontract.Record {
	return issueopsviewcontract.Record{
		ID: "io-full", Phase: issueopscontract.PhasePR,
		Intent:               &issueopscontract.IntentContract{RecordedAt: "2026-09-01T00:00:00Z"},
		ArchitectureReview:   &reviewcontract.ArchitectureReview{Approved: true, ReviewedArchitectureDigest: "sha256:architecture", ReviewedAt: "2026-09-02T00:00:00Z"},
		PlanPath:             "plan.md",
		DevilsAdvocateReview: &issueopscontract.DevilsAdvocateReview{Verdict: "pass", ReviewedPlanDigest: "sha256:plan", RecordedAt: "2026-09-03T00:00:00Z"},
		Execution: &issueopscontract.Execution{
			Completion: &issueopscontract.ExecutionCompletion{CompletedAt: "2026-09-08T00:00:00Z"},
		},
		ImplementationReview: &issueopscontract.ImplementationReview{Verdict: "revise", RecordedAt: "2026-09-04T00:00:00Z"},
		AISlopCleanAt:        "2026-09-05T00:00:00Z",
		ProjectDocsReview:    &issueopscontract.ProjectDocsReview{Verdict: "no-change", RecordedAt: "2026-09-06T00:00:00Z"},
		SchemaEvidence:       &issueopscontract.SchemaEvidence{RecordedAt: "2026-09-06T12:00:00Z"},
		RemoteArtifact:       &issueopscontract.RemoteArtifactVerification{URL: "https://example.test/pr/1", VerifiedAt: "2026-09-07T00:00:00Z"},
		Delegation:           &issueopscontract.DelegationContract{ParentCycleID: "io-parent"},
	}
}

// gateRows are the seven binding edges: the gate key `next` reports, the edge
// it belongs to, and how to remove the edge's target node from a full record.
var gateRows = []struct {
	gate, from, to, binding string
	dropTarget              func(*issueopsviewcontract.Record)
}{
	{"architecture_review_stale", "architecture_doc", "architecture_review", "digest", func(r *issueopsviewcontract.Record) {
		r.ArchitectureReview = &reviewcontract.ArchitectureReview{Approved: false, ReviewedArchitectureDigest: "sha256:architecture"}
	}},
	{"devils_advocate_review_stale", "plan", "plan_review", "digest", func(r *issueopsviewcontract.Record) { r.DevilsAdvocateReview = nil }},
	{"implementation_review_stale", "change_set", "implementation_review", "fingerprint", func(r *issueopsviewcontract.Record) { r.ImplementationReview = nil }},
	{"ai_slop_clean_stale", "change_set", "ai_slop_clean", "fingerprint", func(r *issueopsviewcontract.Record) { r.AISlopCleanAt = "" }},
	{"project_docs_review_stale", "change_set", "project_docs_review", "fingerprint", func(r *issueopsviewcontract.Record) { r.ProjectDocsReview = nil }},
	{"schema_evidence_stale", "change_set", "schema_evidence", "fingerprint", func(r *issueopsviewcontract.Record) { r.SchemaEvidence = nil }},
	{"pr_head_stale", "change_set", "pull_request", "head", func(r *issueopsviewcontract.Record) { r.RemoteArtifact = nil }},
}

// unjudgeableNextStages is every `next` stage a graph cannot judge, written out
// here so a stage added to `next` or moved between the two lists fails
// TestGraphStagesClassifyEveryNextStage.
var unjudgeableNextStages = []string{
	issueopsnextcontract.StageNone, issueopsnextcontract.StageAmbiguous, issueopsnextcontract.StageInvalid,
	issueopsnextcontract.StageBlockedPending, issueopsnextcontract.StageBlockedRoot, issueopsnextcontract.StageBlockedHolderLive,
	issueopsnextcontract.StageTakeover, issueopsnextcontract.StageClaim, issueopsnextcontract.StageIssue,
	issueopsnextcontract.StagePrepare, issueopsnextcontract.StageDone, issueopsnextcontract.StageCleanupResume,
	issueopsnextcontract.StageUnknown,
}

// allNextStages is the `Stage*` constants of the next contract.
var allNextStages = []string{
	issueopsnextcontract.StageNone, issueopsnextcontract.StageAmbiguous, issueopsnextcontract.StageInvalid,
	issueopsnextcontract.StageBlockedPending, issueopsnextcontract.StageBlockedRoot, issueopsnextcontract.StageBlockedHolderLive,
	issueopsnextcontract.StageTakeover, issueopsnextcontract.StageClaim, issueopsnextcontract.StageIssue,
	issueopsnextcontract.StagePrepare, issueopsnextcontract.StagePlanWrite, issueopsnextcontract.StagePlanArchitecture,
	issueopsnextcontract.StagePlanReview, issueopsnextcontract.StagePlanHandoff, issueopsnextcontract.StageImplementEnter,
	issueopsnextcontract.StageImplement, issueopsnextcontract.StageClean, issueopsnextcontract.StageDocs,
	issueopsnextcontract.StageVerify, issueopsnextcontract.StageCommitPush, issueopsnextcontract.StagePRCreate,
	issueopsnextcontract.StagePRComplete, issueopsnextcontract.StageDone, issueopsnextcontract.StageCleanupResume,
	issueopsnextcontract.StageUnknown,
}

// Every stage `next` can answer is either judgeable or not, never both and
// never neither. A new Stage constant fails the count below until someone
// classifies it here and in judgeableStages.
func TestGraphStagesClassifyEveryNextStage(t *testing.T) {
	for _, stage := range allNextStages {
		judgeable, unjudgeable := slices.Contains(judgeableStages, stage), slices.Contains(unjudgeableNextStages, stage)
		if judgeable == unjudgeable {
			t.Errorf("stage %q: in judgeableStages = %t, in the unjudgeable list = %t, want exactly one", stage, judgeable, unjudgeable)
		}
	}

	file, err := parser.ParseFile(token.NewFileSet(), "../../contract/issueopsnext/types.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	constants := 0
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.CONST {
			continue
		}
		for _, spec := range generic.Specs {
			for _, name := range spec.(*ast.ValueSpec).Names {
				if strings.HasPrefix(name.Name, "Stage") {
					constants++
				}
			}
		}
	}
	if constants != len(allNextStages) {
		t.Errorf("the next contract declares %d Stage constants, allNextStages lists %d: classify the new stage", constants, len(allNextStages))
	}
	if sum := len(judgeableStages) + len(unjudgeableNextStages); sum != constants {
		t.Errorf("judgeable %d + unjudgeable %d = %d stages, the next contract declares %d Stage constants", len(judgeableStages), len(unjudgeableNextStages), sum, constants)
	}
}

func bindingEdges(graph issueopsviewcontract.Graph) []issueopsviewcontract.GraphEdge {
	var edges []issueopsviewcontract.GraphEdge
	for _, edge := range graph.Edges {
		if edge.Binding != "" {
			edges = append(edges, edge)
		}
	}
	return edges
}

func edgeStates(edges []issueopsviewcontract.GraphEdge) map[string]string {
	states := map[string]string{}
	for _, edge := range edges {
		states[edge.Gate] = edge.State
	}
	return states
}

func assertAllBindingStates(t *testing.T, graph issueopsviewcontract.Graph, want string) {
	t.Helper()
	edges := bindingEdges(graph)
	if len(edges) != len(gateRows) {
		t.Fatalf("binding edges = %d, want %d: %+v", len(edges), len(gateRows), edges)
	}
	for _, edge := range edges {
		if edge.State != want {
			t.Fatalf("edge %s -> %s state = %q, want %q", edge.From, edge.To, edge.State, want)
		}
	}
}

func TestEvidenceGraph(t *testing.T) {
	judged := func(missing ...string) Judgment { return Judgment{Stage: "verify", Missing: missing} }

	for _, row := range gateRows {
		t.Run("stale "+row.gate, func(t *testing.T) {
			graph := EvidenceGraph(fullGraphRecord(), judged(row.gate))
			states := edgeStates(bindingEdges(graph))
			if len(states) != 7 {
				t.Fatalf("binding gates = %v, want 7", states)
			}
			for gate, state := range states {
				want := issueopsviewcontract.EdgeStateNotReported
				if gate == row.gate {
					want = issueopsviewcontract.EdgeStateStale
				}
				if state != want {
					t.Fatalf("gate %s state = %q, want %q (stale gate %s)", gate, state, want, row.gate)
				}
			}
			var stale []issueopsviewcontract.GraphEdge
			for _, edge := range graph.Edges {
				if edge.State == issueopsviewcontract.EdgeStateStale {
					stale = append(stale, edge)
				}
			}
			wantEdge := issueopsviewcontract.GraphEdge{From: row.from, To: row.to, Binding: row.binding, Gate: row.gate, State: "stale"}
			if !reflect.DeepEqual(stale, []issueopsviewcontract.GraphEdge{wantEdge}) {
				t.Fatalf("stale edges = %+v, want %+v", stale, wantEdge)
			}
			if !reflect.DeepEqual(graph.JudgedBy.StaleKeys, []string{row.gate}) {
				t.Fatalf("stale_keys = %v", graph.JudgedBy.StaleKeys)
			}
			if graph.JudgedBy.UnmappedStaleKeys == nil || len(graph.JudgedBy.UnmappedStaleKeys) != 0 {
				t.Fatalf("unmapped_stale_keys = %#v, want an empty non-nil slice", graph.JudgedBy.UnmappedStaleKeys)
			}
			if graph.UnknownReason != "" || graph.JudgedBy.Stage != "verify" {
				t.Fatalf("judged_by = %+v reason=%q", graph.JudgedBy, graph.UnknownReason)
			}
		})
		t.Run("target absent "+row.gate, func(t *testing.T) {
			record := fullGraphRecord()
			row.dropTarget(&record)
			graph := EvidenceGraph(record, judged(row.gate))
			for _, edge := range graph.Edges {
				if edge.Gate == row.gate {
					t.Fatalf("edge %+v is drawn although its target is absent", edge)
				}
			}
			if len(bindingEdges(graph)) != 6 {
				t.Fatalf("binding edges = %+v, want the other 6", bindingEdges(graph))
			}
			if !reflect.DeepEqual(graph.JudgedBy.StaleKeys, []string{row.gate}) || !reflect.DeepEqual(graph.JudgedBy.UnmappedStaleKeys, []string{row.gate}) {
				t.Fatalf("judged_by = %+v, want %s in stale_keys and unmapped_stale_keys", graph.JudgedBy, row.gate)
			}
			for _, edge := range bindingEdges(graph) {
				if edge.State != issueopsviewcontract.EdgeStateNotReported {
					t.Fatalf("edge %+v must stay not_reported when only the absent edge's key is missing", edge)
				}
			}
		})
	}

	t.Run("a stale key the table does not know is unmapped", func(t *testing.T) {
		graph := EvidenceGraph(fullGraphRecord(), judged("foo_stale"))
		if !reflect.DeepEqual(graph.JudgedBy.StaleKeys, []string{"foo_stale"}) || !reflect.DeepEqual(graph.JudgedBy.UnmappedStaleKeys, []string{"foo_stale"}) {
			t.Fatalf("judged_by = %+v", graph.JudgedBy)
		}
		assertAllBindingStates(t, graph, issueopsviewcontract.EdgeStateNotReported)
	})

	t.Run("stale keys are sorted and unmapped keeps only the undrawn ones", func(t *testing.T) {
		record := fullGraphRecord()
		record.SchemaEvidence = nil
		graph := EvidenceGraph(record, judged("pr_head_stale", "schema_evidence_stale", "aaa_stale", "pr_head_stale"))
		if want := []string{"aaa_stale", "pr_head_stale", "schema_evidence_stale"}; !reflect.DeepEqual(graph.JudgedBy.StaleKeys, want) {
			t.Fatalf("stale_keys = %v, want %v", graph.JudgedBy.StaleKeys, want)
		}
		if want := []string{"aaa_stale", "schema_evidence_stale"}; !reflect.DeepEqual(graph.JudgedBy.UnmappedStaleKeys, want) {
			t.Fatalf("unmapped_stale_keys = %v, want %v", graph.JudgedBy.UnmappedStaleKeys, want)
		}
	})

	t.Run("an empty missing reports every binding edge not_reported", func(t *testing.T) {
		for _, missing := range [][]string{nil, {}} {
			graph := EvidenceGraph(fullGraphRecord(), Judgment{Stage: "verify", Missing: missing})
			assertAllBindingStates(t, graph, issueopsviewcontract.EdgeStateNotReported)
			if graph.JudgedBy.StaleKeys == nil || len(graph.JudgedBy.StaleKeys) != 0 || graph.JudgedBy.UnmappedStaleKeys == nil {
				t.Fatalf("judged_by = %#v, want empty non-nil lists", graph.JudgedBy)
			}
		}
	})

	t.Run("a missing key that is not stale marks nothing stale", func(t *testing.T) {
		graph := EvidenceGraph(fullGraphRecord(), judged("implementation_review"))
		assertAllBindingStates(t, graph, issueopsviewcontract.EdgeStateNotReported)
		if len(graph.JudgedBy.StaleKeys) != 0 || len(graph.JudgedBy.UnmappedStaleKeys) != 0 || graph.JudgedBy.StaleKeys == nil {
			t.Fatalf("judged_by = %#v, want empty non-nil lists", graph.JudgedBy)
		}
	})

	t.Run("a failed next makes every binding edge unknown", func(t *testing.T) {
		graph := EvidenceGraph(fullGraphRecord(), Judgment{Stage: "verify", NextFailed: true, WorktreeMissing: true})
		assertAllBindingStates(t, graph, issueopsviewcontract.EdgeStateUnknown)
		if graph.UnknownReason != issueopsviewcontract.UnknownNextFailed {
			t.Fatalf("unknown_reason = %q, want next_failed ahead of every other reason", graph.UnknownReason)
		}
	})

	for _, stage := range append([]string{"", "future.stage"}, unjudgeableNextStages...) {
		t.Run(fmt.Sprintf("stage %q is unjudgeable", stage), func(t *testing.T) {
			graph := EvidenceGraph(fullGraphRecord(), Judgment{Stage: stage, WorktreeMissing: true})
			assertAllBindingStates(t, graph, issueopsviewcontract.EdgeStateUnknown)
			if graph.UnknownReason != issueopsviewcontract.UnknownStageUnjudgeable {
				t.Fatalf("unknown_reason = %q, want stage_unjudgeable ahead of worktree_missing", graph.UnknownReason)
			}
			if graph.JudgedBy.Stage != stage {
				t.Fatalf("judged_by.stage = %q, want %q", graph.JudgedBy.Stage, stage)
			}
		})
	}

	t.Run("a judgeable stage judges every binding edge", func(t *testing.T) {
		for _, stage := range judgeableStages {
			graph := EvidenceGraph(fullGraphRecord(), Judgment{Stage: stage})
			assertAllBindingStates(t, graph, issueopsviewcontract.EdgeStateNotReported)
			if graph.UnknownReason != "" {
				t.Fatalf("stage %q: unknown_reason = %q", stage, graph.UnknownReason)
			}
		}
	})

	t.Run("merge_in_progress in missing makes a judgeable stage unjudgeable", func(t *testing.T) {
		graph := EvidenceGraph(fullGraphRecord(), Judgment{Stage: "implement", Missing: []string{"merge_in_progress"}})
		assertAllBindingStates(t, graph, issueopsviewcontract.EdgeStateUnknown)
		if graph.UnknownReason != issueopsviewcontract.UnknownStageUnjudgeable {
			t.Fatalf("unknown_reason = %q", graph.UnknownReason)
		}
	})

	t.Run("a missing worktree leaves the digest edges judged and the worktree edges unknown", func(t *testing.T) {
		graph := EvidenceGraph(fullGraphRecord(), Judgment{Stage: "verify", WorktreeMissing: true})
		if graph.UnknownReason != issueopsviewcontract.UnknownWorktreeMissing {
			t.Fatalf("unknown_reason = %q, want worktree_missing", graph.UnknownReason)
		}
		want := map[string]string{
			"architecture_review_stale":    issueopsviewcontract.EdgeStateNotReported,
			"devils_advocate_review_stale": issueopsviewcontract.EdgeStateNotReported,
			"implementation_review_stale":  issueopsviewcontract.EdgeStateUnknown,
			"ai_slop_clean_stale":          issueopsviewcontract.EdgeStateUnknown,
			"project_docs_review_stale":    issueopsviewcontract.EdgeStateUnknown,
			"schema_evidence_stale":        issueopsviewcontract.EdgeStateUnknown,
			"pr_head_stale":                issueopsviewcontract.EdgeStateUnknown,
		}
		if got := edgeStates(bindingEdges(graph)); !reflect.DeepEqual(got, want) {
			t.Fatalf("edge states = %v, want %v", got, want)
		}
	})

	t.Run("a stale gate stays stale on an unjudgeable stage, and the other edges are unknown", func(t *testing.T) {
		graph := EvidenceGraph(fullGraphRecord(), Judgment{Stage: "prepare", Missing: []string{"architecture_review_stale"}})
		if graph.UnknownReason != issueopsviewcontract.UnknownStageUnjudgeable {
			t.Fatalf("unknown_reason = %q, want stage_unjudgeable", graph.UnknownReason)
		}
		states := edgeStates(bindingEdges(graph))
		if len(states) != len(gateRows) {
			t.Fatalf("edge states = %v, want %d binding edges", states, len(gateRows))
		}
		for gate, state := range states {
			want := issueopsviewcontract.EdgeStateUnknown
			if gate == "architecture_review_stale" {
				want = issueopsviewcontract.EdgeStateStale
			}
			if state != want {
				t.Fatalf("gate %s state = %q, want %q", gate, state, want)
			}
		}
		if !reflect.DeepEqual(graph.JudgedBy.StaleKeys, []string{"architecture_review_stale"}) {
			t.Fatalf("stale_keys = %v, want the stale edge's gate", graph.JudgedBy.StaleKeys)
		}
	})

	t.Run("a stale worktree edge stays stale when the worktree is missing", func(t *testing.T) {
		graph := EvidenceGraph(fullGraphRecord(), Judgment{Stage: "verify", Missing: []string{"implementation_review_stale"}, WorktreeMissing: true})
		if graph.UnknownReason != issueopsviewcontract.UnknownWorktreeMissing {
			t.Fatalf("unknown_reason = %q, want worktree_missing", graph.UnknownReason)
		}
		states := edgeStates(bindingEdges(graph))
		if states["implementation_review_stale"] != issueopsviewcontract.EdgeStateStale {
			t.Fatalf("implementation_review_stale state = %q, want stale", states["implementation_review_stale"])
		}
		for _, gate := range []string{"ai_slop_clean_stale", "project_docs_review_stale", "schema_evidence_stale", "pr_head_stale"} {
			if states[gate] != issueopsviewcontract.EdgeStateUnknown {
				t.Fatalf("%s state = %q, want unknown without a worktree", gate, states[gate])
			}
		}
		for _, gate := range []string{"architecture_review_stale", "devils_advocate_review_stale"} {
			if states[gate] != issueopsviewcontract.EdgeStateNotReported {
				t.Fatalf("%s state = %q, want not_reported: a digest edge does not need the worktree", gate, states[gate])
			}
		}
	})

	t.Run("an empty record has only the cycle node and no edges", func(t *testing.T) {
		graph := EvidenceGraph(issueopsviewcontract.Record{}, Judgment{})
		var present []string
		for _, node := range graph.Nodes {
			if node.Present {
				present = append(present, node.ID)
			}
		}
		if !reflect.DeepEqual(present, []string{"cycle"}) {
			t.Fatalf("present nodes = %v, want only cycle", present)
		}
		if len(graph.Edges) != 0 || graph.Edges == nil {
			t.Fatalf("edges = %#v, want an empty non-nil slice", graph.Edges)
		}
		if graph.ChildrenTotal != 0 || graph.ChildrenShown != 0 {
			t.Fatalf("children = %d/%d", graph.ChildrenShown, graph.ChildrenTotal)
		}
		encoded, err := json.Marshal(graph)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{`"edges":[]`, `"stale_keys":[]`, `"unmapped_stale_keys":[]`, `"limits":["next_reports_first_blocking_gate_only","document_nodes_from_review_records"]`} {
			if !strings.Contains(string(encoded), want) {
				t.Fatalf("graph JSON lacks %s: %s", want, encoded)
			}
		}
	})

	t.Run("a full record lists every node in order with the edge table's edges", func(t *testing.T) {
		record := fullGraphRecord()
		record.ChildCycles = []issueopscontract.ChildCycleRef{
			{CycleID: "io-c1", ValidationVerdict: "accepted", ChildIssueURL: "https://example.test/issues/9"},
			{CycleID: "io-c2"},
		}
		graph := EvidenceGraph(record, judged())
		wantIDs := []string{
			"cycle", "intent", "architecture_doc", "architecture_review", "plan", "plan_review", "change_set",
			"implementation_review", "ai_slop_clean", "project_docs_review", "schema_evidence", "pull_request",
			"completion", "parent", "child:io-c1", "child:io-c2",
		}
		var gotIDs []string
		for _, node := range graph.Nodes {
			gotIDs = append(gotIDs, node.ID)
			if !node.Present {
				t.Fatalf("node %s is absent in a full record", node.ID)
			}
		}
		if !reflect.DeepEqual(gotIDs, wantIDs) {
			t.Fatalf("node ids = %v, want %v", gotIDs, wantIDs)
		}
		byID := map[string]issueopsviewcontract.GraphNode{}
		for _, node := range graph.Nodes {
			byID[node.ID] = node
		}
		for id, want := range map[string]issueopsviewcontract.GraphNode{
			"intent":                {ID: "intent", Kind: "intent", Present: true, At: "2026-09-01T00:00:00Z"},
			"architecture_review":   {ID: "architecture_review", Kind: "architecture_review", Present: true, At: "2026-09-02T00:00:00Z"},
			"plan_review":           {ID: "plan_review", Kind: "plan_review", Present: true, At: "2026-09-03T00:00:00Z", Verdict: "pass"},
			"implementation_review": {ID: "implementation_review", Kind: "implementation_review", Present: true, At: "2026-09-04T00:00:00Z", Verdict: "revise"},
			"ai_slop_clean":         {ID: "ai_slop_clean", Kind: "ai_slop_clean", Present: true, At: "2026-09-05T00:00:00Z"},
			"project_docs_review":   {ID: "project_docs_review", Kind: "project_docs_review", Present: true, At: "2026-09-06T00:00:00Z", Verdict: "no-change"},
			"schema_evidence":       {ID: "schema_evidence", Kind: "schema_evidence", Present: true, At: "2026-09-06T12:00:00Z"},
			"pull_request":          {ID: "pull_request", Kind: "pull_request", Present: true, At: "2026-09-07T00:00:00Z", URL: "https://example.test/pr/1"},
			"completion":            {ID: "completion", Kind: "completion", Present: true, At: "2026-09-08T00:00:00Z"},
			"child:io-c1":           {ID: "child:io-c1", Kind: "child", Present: true, Verdict: "accepted", URL: "https://example.test/issues/9"},
		} {
			if byID[id] != want {
				t.Fatalf("node %s = %+v, want %+v", id, byID[id], want)
			}
		}

		type pair struct{ from, to string }
		var gotEdges []pair
		for _, edge := range graph.Edges {
			gotEdges = append(gotEdges, pair{edge.From, edge.To})
			if edge.Binding == "" && (edge.State != "" || edge.Gate != "") {
				t.Fatalf("structural edge %+v carries a gate or state", edge)
			}
		}
		wantEdges := []pair{
			{"intent", "architecture_doc"}, {"architecture_doc", "architecture_review"}, {"architecture_review", "plan"},
			{"plan", "plan_review"}, {"plan_review", "change_set"}, {"change_set", "implementation_review"},
			{"change_set", "ai_slop_clean"}, {"change_set", "project_docs_review"}, {"change_set", "schema_evidence"},
			{"change_set", "pull_request"}, {"implementation_review", "pull_request"}, {"pull_request", "completion"},
			{"parent", "cycle"}, {"cycle", "child:io-c1"}, {"cycle", "child:io-c2"},
		}
		if !reflect.DeepEqual(gotEdges, wantEdges) {
			t.Fatalf("edges = %v, want %v", gotEdges, wantEdges)
		}
		if graph.Phase != "pr" || graph.ChildrenTotal != 2 || graph.ChildrenShown != 2 {
			t.Fatalf("graph header = phase %q children %d/%d", graph.Phase, graph.ChildrenShown, graph.ChildrenTotal)
		}
	})

	t.Run("an edge needs both of its nodes", func(t *testing.T) {
		record := fullGraphRecord()
		record.Execution = nil
		record.Delegation = nil
		graph := EvidenceGraph(record, judged("implementation_review_stale"))
		var got []string
		for _, edge := range graph.Edges {
			got = append(got, edge.From+"->"+edge.To)
		}
		want := []string{
			"intent->architecture_doc", "architecture_doc->architecture_review", "architecture_review->plan",
			"plan->plan_review", "implementation_review->pull_request",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("edges = %v, want %v (no edge may touch an absent node)", got, want)
		}
		if !reflect.DeepEqual(graph.JudgedBy.UnmappedStaleKeys, []string{"implementation_review_stale"}) {
			t.Fatalf("unmapped_stale_keys = %v, want the undrawn gate", graph.JudgedBy.UnmappedStaleKeys)
		}
	})

	t.Run("children beyond the cap are counted but not drawn", func(t *testing.T) {
		record := issueopsviewcontract.Record{}
		for i := range 55 {
			record.ChildCycles = append(record.ChildCycles, issueopscontract.ChildCycleRef{CycleID: fmt.Sprintf("io-c%02d", i)})
		}
		graph := EvidenceGraph(record, Judgment{})
		if graph.ChildrenTotal != 55 || graph.ChildrenShown != 50 {
			t.Fatalf("children = %d shown of %d total, want 50 of 55", graph.ChildrenShown, graph.ChildrenTotal)
		}
		children, childEdges := 0, 0
		for _, node := range graph.Nodes {
			if strings.HasPrefix(node.ID, "child:") {
				children++
			}
		}
		for _, edge := range graph.Edges {
			if strings.HasPrefix(edge.To, "child:") {
				childEdges++
			}
		}
		if children != 50 || childEdges != 50 {
			t.Fatalf("child nodes = %d, child edges = %d, want 50 and 50", children, childEdges)
		}
		if last := graph.Nodes[len(graph.Nodes)-1].ID; last != "child:io-c49" {
			t.Fatalf("last child = %s, want the 50th in record order", last)
		}
	})

	t.Run("limits always name both limits", func(t *testing.T) {
		want := []string{"next_reports_first_blocking_gate_only", "document_nodes_from_review_records"}
		for _, judgment := range []Judgment{{}, {Stage: "verify"}, {NextFailed: true}, {Stage: "done"}} {
			graph := EvidenceGraph(fullGraphRecord(), judgment)
			if !reflect.DeepEqual(graph.Limits, want) {
				t.Fatalf("limits = %v for %+v, want %v", graph.Limits, judgment, want)
			}
		}
	})

	t.Run("document nodes follow the review records, not approval", func(t *testing.T) {
		present := func(record issueopsviewcontract.Record) map[string]bool {
			found := map[string]bool{}
			for _, node := range EvidenceGraph(record, Judgment{}).Nodes {
				found[node.ID] = node.Present
			}
			return found
		}
		cases := []struct {
			name                     string
			record                   issueopsviewcontract.Record
			wantArchitecture         bool
			wantArchitectureApproved bool
			wantPlan                 bool
		}{
			{name: "no review and no plan path", record: issueopsviewcontract.Record{}},
			{
				name:             "an unapproved review bound to a digest still shows the document",
				record:           issueopsviewcontract.Record{ArchitectureReview: &reviewcontract.ArchitectureReview{ReviewedArchitectureDigest: "sha256:a"}},
				wantArchitecture: true,
			},
			{
				// Approved, but nothing says which document: the review node is
				// present and the document node is not.
				name:                     "a review without any digest shows no document",
				record:                   issueopsviewcontract.Record{ArchitectureReview: &reviewcontract.ArchitectureReview{Approved: true}},
				wantArchitectureApproved: true,
			},
			{
				name: "an earlier round's digest is enough",
				record: issueopsviewcontract.Record{ArchitectureReview: &reviewcontract.ArchitectureReview{
					Approved: true,
					History:  []reviewcontract.ArchitectureReviewRound{{ReviewedArchitectureDigest: "sha256:old"}},
				}},
				wantArchitecture: true, wantArchitectureApproved: true,
			},
			{name: "a plan path is a plan", record: issueopsviewcontract.Record{PlanPath: "plan.md"}, wantPlan: true},
			{
				name:     "a plan review bound to a digest is a plan",
				record:   issueopsviewcontract.Record{DevilsAdvocateReview: &issueopscontract.DevilsAdvocateReview{Verdict: "revise", ReviewedPlanDigest: "sha256:p"}},
				wantPlan: true,
			},
			{
				name: "an earlier plan round's digest is a plan",
				record: issueopsviewcontract.Record{DevilsAdvocateReview: &issueopscontract.DevilsAdvocateReview{
					Verdict: "pass", History: []issueopscontract.DevilsAdvocateRound{{ReviewedPlanDigest: "sha256:old"}},
				}},
				wantPlan: true,
			},
			{
				name:   "a plan review without a digest or a plan path shows no plan",
				record: issueopsviewcontract.Record{DevilsAdvocateReview: &issueopscontract.DevilsAdvocateReview{Verdict: "pass"}},
			},
		}
		for _, tc := range cases {
			got := present(tc.record)
			if got["architecture_doc"] != tc.wantArchitecture || got["architecture_review"] != tc.wantArchitectureApproved || got["plan"] != tc.wantPlan {
				t.Errorf("%s: architecture_doc=%t architecture_review=%t plan=%t, want %t %t %t",
					tc.name, got["architecture_doc"], got["architecture_review"], got["plan"],
					tc.wantArchitecture, tc.wantArchitectureApproved, tc.wantPlan)
			}
		}
	})
}
