package issueopscli

import (
	"encoding/json"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	issueopscore "issueops/internal/adapter/issueops"
	issueopscontract "issueops/internal/contract/issueops"
	issueopsnextcontract "issueops/internal/contract/issueopsnext"
	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsviewcontract "issueops/internal/contract/issueopsview"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/testsupport"
)

// clearViewGraphSessionEnv removes every native session variable so `next`
// resolves no actor and never reports an ambiguous host identity.
func clearViewGraphSessionEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{"CLAUDE_CODE_SESSION_ID", "CODEX_THREAD_ID", "PI_SESSION_ID", "ISSUEOPS_OMP_SESSION_ID", "OPENCODE_SESSION_ID"} {
		t.Setenv(name, "")
	}
}

// initViewGraphRepo makes repo a git repository with one empty commit and
// returns that commit's SHA.
func initViewGraphRepo(t *testing.T, repo string) string {
	t.Helper()
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", repo, "-c", "user.name=t", "-c", "user.email=t@example.test"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	git("commit", "-q", "--allow-empty", "-m", "base")
	return git("rev-parse", "HEAD")
}

// graphStaleGates returns the sorted gates of the edges the graph marks stale.
func graphStaleGates(graph *issueopsviewcontract.Graph) []string {
	gates := []string{}
	for _, edge := range graph.Edges {
		if edge.State == issueopsviewcontract.EdgeStateStale {
			gates = append(gates, edge.Gate)
		}
	}
	slices.Sort(gates)
	return gates
}

// nextStaleKeys returns the sorted `_stale` keys of a next result's missing.
func nextStaleKeys(missing []string) []string {
	keys := []string{}
	for _, key := range missing {
		if strings.HasSuffix(key, "_stale") {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return keys
}

// 그래프의 stale edge는 같은 state에서 `next`가 보고한 `_stale` 키와 같다. 아키텍처
// 문서와 plan을 승인 뒤에 고치는 흐름을 단계마다 `next`와 `view --view graph`로 읽고,
// 기대 집합도 따로 단정한다.
func TestViewGraphStaleMatchesNext(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	clearViewGraphSessionEnv(t)
	repo := makeIssueOpsCLIRepoForTest(t, "example")
	head := initViewGraphRepo(t, repo)
	var started map[string]any
	if err := json.Unmarshal([]byte(captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", "1-provider-linked-branch", "--json"})
	})), &started); err != nil {
		t.Fatal(err)
	}
	id := started["id"].(string)
	recordIssueOpsCLIIntentForTest(t, id)
	recordIssueOpsCLIPlanPrepForTest(t, id)
	captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"link-issue", "--id", id, "--issue-url", "https://github.com/example/repo/issues/1", "--json"})
	})
	captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"branch", "prepare", "--id", id, "--provider", "github", "--issue-url", "https://github.com/example/repo/issues/1", "--branch", "1-provider-linked-branch", "--base-branch", "main", "--base-sha", head, "--parent-worktree", repo + ".worktrees/main", "--link-verified", "--json"})
	})

	stage := func(name, body string) {
		t.Helper()
		path := filepath.Join(t.TempDir(), name+".md")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		captureStdoutForContract(t, func() error {
			return runIssueOps([]string{"artifact", "stage", "--id", id, "--name", name, "--file", path, "--json"})
		})
	}
	// read runs next and the graph view against the same state and cwd.
	read := func(cwd string) (issueopsnextcontract.Result, *issueopsviewcontract.Graph) {
		t.Helper()
		var next issueopsnextcontract.Result
		if out := captureStdoutForContract(t, func() error {
			return runIssueOps([]string{"next", "--id", id, "--cwd", cwd, "--json"})
		}); json.Unmarshal([]byte(out), &next) != nil {
			t.Fatalf("next --json is not JSON:\n%s", out)
		}
		var view issueopsviewcontract.Result
		if out := captureStdoutForContract(t, func() error {
			return runIssueOps([]string{"view", "--id", id, "--view", "graph", "--cwd", cwd, "--json"})
		}); json.Unmarshal([]byte(out), &view) != nil || !view.OK || view.Graph == nil || view.View != "graph" {
			t.Fatalf("view --view graph --json = %+v\n%s", view, out)
		}
		return next, view.Graph
	}
	assertStale := func(step string, want []string) *issueopsviewcontract.Graph {
		t.Helper()
		next, graph := read(repo)
		if got := nextStaleKeys(next.Missing); !slices.Equal(got, want) {
			t.Fatalf("%s: next stale keys = %v, want %v (stage %s missing %v)", step, got, want, next.Stage.Key, next.Missing)
		}
		if got := graphStaleGates(graph); !slices.Equal(got, want) {
			t.Fatalf("%s: graph stale gates = %v, want %v", step, got, want)
		}
		if graph.JudgedBy.Stage != next.Stage.Key {
			t.Fatalf("%s: judged_by.stage = %q, next stage = %q", step, graph.JudgedBy.Stage, next.Stage.Key)
		}
		if graph.UnknownReason != "" {
			t.Fatalf("%s: unknown_reason = %q in stage %s", step, graph.UnknownReason, next.Stage.Key)
		}
		return graph
	}
	edgeState := func(graph *issueopsviewcontract.Graph, from, to string) string {
		t.Helper()
		for _, edge := range graph.Edges {
			if edge.From == from && edge.To == to {
				return edge.State
			}
		}
		t.Fatalf("graph has no edge %s -> %s: %+v", from, to, graph.Edges)
		return ""
	}
	architecture := strings.Join(issueopsdomain.RequiredArchitectureSections, "\n본문\n") + "\n본문\n"

	// (a) approved architecture plus a staged plan: nothing is stale.
	stage("plan", "# plan\n"+strings.Join(issueopsdomain.RequiredPlanSections, "\n본문\n")+"\n본문\n")
	stage("architecture", architecture)
	recordIssueOpsCLIArchitectureForTest(t, id)
	graph := assertStale("a", []string{})
	if state := edgeState(graph, "architecture_doc", "architecture_review"); state != issueopsviewcontract.EdgeStateNotReported {
		t.Fatalf("a: architecture_doc -> architecture_review = %q, want not_reported", state)
	}

	// (b) editing the architecture after approval makes the architecture review stale.
	stage("architecture", architecture+"추가한 절충점\n")
	graph = assertStale("b", []string{"architecture_review_stale"})
	if state := edgeState(graph, "architecture_doc", "architecture_review"); state != issueopsviewcontract.EdgeStateStale {
		t.Fatalf("b: architecture_doc -> architecture_review = %q, want stale", state)
	}

	// (c) restoring the document and approving again, then a passing plan review.
	stage("architecture", architecture)
	recordIssueOpsCLIArchitectureForTest(t, id)
	captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"devils-advocate", "review", "--id", id, "--verdict", "pass", "--reviewer-context", "subagent", "--finding", "attacked gate 3: no second caller exists", "--json"})
	})
	graph = assertStale("c", []string{})
	if state := edgeState(graph, "plan", "plan_review"); state != issueopsviewcontract.EdgeStateNotReported {
		t.Fatalf("c: plan -> plan_review = %q, want not_reported", state)
	}

	// (d) editing the plan after its review makes the plan review stale.
	stage("plan", "# plan v2\n"+strings.Join(issueopsdomain.RequiredPlanSections, "\n본문\n")+"\n본문\n")
	graph = assertStale("d", []string{"devils_advocate_review_stale"})
	if state := edgeState(graph, "plan", "plan_review"); state != issueopsviewcontract.EdgeStateStale {
		t.Fatalf("d: plan -> plan_review = %q, want stale", state)
	}

	// (e) from another repository next stops at invalid, so no edge can be judged.
	other := makeIssueOpsCLIRepoForTest(t, "other")
	initViewGraphRepo(t, other)
	next, graph := read(other)
	if next.Stage.Key != "invalid" {
		t.Fatalf("e: next stage = %q, want invalid (missing %v)", next.Stage.Key, next.Missing)
	}
	if graph.JudgedBy.Stage != "invalid" || graph.UnknownReason != issueopsviewcontract.UnknownStageUnjudgeable {
		t.Fatalf("e: judged_by = %+v reason = %q", graph.JudgedBy, graph.UnknownReason)
	}
	binding := 0
	for _, edge := range graph.Edges {
		if edge.Binding == "" {
			continue
		}
		binding++
		if edge.State != issueopsviewcontract.EdgeStateUnknown {
			t.Fatalf("e: edge %+v must be unknown", edge)
		}
	}
	if binding == 0 {
		t.Fatalf("e: the graph has no binding edge to judge: %+v", graph.Edges)
	}
}

// execution의 worktree 경로가 없는 record는 `next`가 판정 가능한 stage를 내도 worktree에 묶인
// binding edge(fingerprint, head)를 알 수 없다. 문서 digest edge는 worktree와 무관하므로 계속
// 판정한다. record는 1단계 helper로 state에 직접 넣고 실제 CLI 경로로 읽는다.
func TestViewGraphUnknownWithoutWorktree(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	clearViewGraphSessionEnv(t)
	const session = "view-graph-session"
	t.Setenv("CODEX_THREAD_ID", session)
	root := issueOpsStateRootForTest()
	repo := testsupport.AbsPath("repo-view-graph")
	receipt, err := issueopscore.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	missingRoot := filepath.Join(t.TempDir(), "no-such-worktree")
	id := "io-view-graph-worktree"
	seedListRecord(t, root, issueopscontract.Record{
		SchemaVersion: issueopscontract.SchemaVersion,
		ID:            id, Repo: repo, Branch: "1-view-graph", Phase: issueopscontract.PhasePlan,
		CreatedAt: "2026-11-01T00:00:00Z", UpdatedAt: "2026-11-01T00:00:00Z",
		IssueURL:             "https://github.com/example/repo/issues/1",
		BranchPrepare:        &issueopscontract.BranchPrepare{Provider: "github", BaseSHA: strings.Repeat("a", 40), LinkVerified: true},
		ArchitectureReview:   &reviewcontract.ArchitectureReview{Approved: true, ReviewedArchitectureDigest: "sha256:architecture", ReviewedAt: "2026-11-01T00:00:00Z"},
		ImplementationReview: &issueopscontract.ImplementationReview{Verdict: "pass", RecordedAt: "2026-11-01T00:00:00Z"},
		Execution: &issueopscontract.Execution{
			Workspace: issueopscontract.Workspace{SourceRoot: repo, Root: missingRoot, Branch: "1-view-graph", BaseHead: strings.Repeat("a", 40), LinkedAt: "2026-11-01T00:00:00Z"},
			Lease: issueopscontract.WriteLease{
				Generation: 1, Status: issueopscontract.LeaseStatusActive, ClaimedAt: "2026-11-01T00:00:00Z",
				Holder: &issueopscontract.NativeActor{Host: "codex", SessionID: session, SessionProcess: &receipt},
			},
		},
	})

	var runErr error
	out := captureStdoutForContract(t, func() error {
		runErr = runIssueOps([]string{"view", "--id", id, "--view", "graph", "--cwd", repo, "--json"})
		return runErr
	})
	if runErr != nil {
		t.Fatalf("view --view graph: %v\n%s", runErr, out)
	}
	var result issueopsviewcontract.Result
	if err := json.Unmarshal([]byte(out), &result); err != nil || !result.OK || result.Graph == nil {
		t.Fatalf("view --json = %+v err=%v\n%s", result, err, out)
	}
	graph := result.Graph
	if graph.UnknownReason != issueopsviewcontract.UnknownWorktreeMissing {
		t.Fatalf("unknown_reason = %q (judged_by %+v), want worktree_missing\n%s", graph.UnknownReason, graph.JudgedBy, out)
	}
	states := map[string]string{}
	for _, edge := range graph.Edges {
		if edge.Binding != "" {
			states[edge.Gate] = edge.State
		}
	}
	want := map[string]string{
		"architecture_review_stale":   issueopsviewcontract.EdgeStateNotReported,
		"implementation_review_stale": issueopsviewcontract.EdgeStateUnknown,
	}
	if !maps.Equal(states, want) {
		t.Fatalf("binding edge states = %v, want %v (the digest edge is judged, the fingerprint edge is not)\n%s", states, want, out)
	}

	text := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"view", "--id", id, "--view", "graph", "--cwd", repo})
	})
	first, _, _ := strings.Cut(text, "\n")
	if first != "graph: stage="+graph.JudgedBy.Stage+" stale=0 unknown=1" {
		t.Fatalf("first text line = %q\n%s", first, text)
	}
	for _, row := range []string{
		"  architecture_doc -> architecture_review  [not_reported]  architecture_review_stale\n",
		"  change_set -> implementation_review  [unknown]  implementation_review_stale\n",
	} {
		if !strings.Contains(text, row) {
			t.Fatalf("text lacks the binding edge row %q:\n%s", row, text)
		}
	}
}
