package issueopsapp

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	issueopscore "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/outbound/issueopsrecord"
	"issueops/internal/adapter/outbound/sqlstore"
	issueopscontract "issueops/internal/contract/issueops"
	issueopsnextcontract "issueops/internal/contract/issueopsnext"
	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsviewcontract "issueops/internal/contract/issueopsview"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
	"issueops/internal/testsupport"

	_ "modernc.org/sqlite"
)

// viewGraphFixture is a started cycle in a real git repository that has an
// issue link and prepared branch, so `next` reaches the plan stages. Every
// command runs through the production composition in runIssueOps.
type viewGraphFixture struct {
	t            *testing.T
	repo, id     string
	architecture string
}

func newViewGraphFixture(t *testing.T) *viewGraphFixture {
	t.Helper()
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	for _, name := range []string{"CLAUDE_CODE_SESSION_ID", "CODEX_THREAD_ID", "PI_SESSION_ID", "ISSUEOPS_OMP_SESSION_ID", "OPENCODE_SESSION_ID"} {
		t.Setenv(name, "")
	}
	repo := makeGitRepoForContract(t)
	head := strings.TrimSpace(gitOutputForViewGraph(t, repo, "rev-parse", "HEAD"))
	fixture := &viewGraphFixture{
		t: t, repo: repo,
		architecture: strings.Join(issueopsdomain.RequiredArchitectureSections, "\n본문\n") + "\n본문\n",
	}
	var started struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(fixture.run("start", "--repo", repo, "--branch", "1-provider-linked-branch", "--json")), &started); err != nil || started.ID == "" {
		t.Fatalf("start did not return an id: %v", err)
	}
	fixture.id = started.ID
	fixture.run("intent", "record", "--id", fixture.id, "--raw-request", "view graph", "--interpreted-intent", "judge the graph with next",
		"--success-criteria", "graph matches next", "--json")
	fixture.run("plan-prep", "record", "--id", fixture.id, "--decisions-evidence", ".issueops/ADR.md", "--related-score-ref", "remote score: selected=#1(0.81), threshold=0.70",
		"--web-research-evidence", ".issueops/research/view.md", "--codebase-survey-evidence", "rg sweep of the view vertical", "--json")
	fixture.run("link-issue", "--id", fixture.id, "--issue-url", "https://github.com/example/repo/issues/1", "--json")
	fixture.run("branch", "prepare", "--id", fixture.id, "--provider", "github", "--issue-url", "https://github.com/example/repo/issues/1",
		"--branch", "1-provider-linked-branch", "--base-branch", "main", "--base-sha", head, "--parent-worktree", repo+".worktrees/main", "--link-verified", "--json")
	return fixture
}

func gitOutputForViewGraph(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func (fixture *viewGraphFixture) run(args ...string) string {
	fixture.t.Helper()
	return captureStdoutForContract(fixture.t, func() error { return runIssueOps(args) })
}

func (fixture *viewGraphFixture) stage(name, body string) {
	fixture.t.Helper()
	path := filepath.Join(fixture.t.TempDir(), name+".md")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		fixture.t.Fatal(err)
	}
	fixture.run("artifact", "stage", "--id", fixture.id, "--name", name, "--file", path, "--json")
}

// approveArchitecture stages architecture (a body, or the fixture's default
// when empty) and records an approved review bound to it.
func (fixture *viewGraphFixture) approveArchitecture(body string) {
	fixture.t.Helper()
	if body == "" {
		body = fixture.architecture
	}
	fixture.stage("architecture", body)
	fixture.run("architecture", "review", "--id", fixture.id,
		"--problem-summary", "view graph must follow next", "--proposed-architecture", "reuse the next handler",
		"--refactor-plan", "none", "--alternative", "recompute staleness", "--risk", "drift from next",
		"--verification", "architecture review checked alternatives and risks", "--verification", "go test ./cmd/issueops/issueopsapp",
		"--reviewer-context", "subagent", "--approved", "--json")
}

// stagePlan stages a plan document whose title is title.
func (fixture *viewGraphFixture) stagePlan(title string) {
	fixture.t.Helper()
	fixture.stage("plan", title+"\n"+strings.Join(issueopsdomain.RequiredPlanSections, "\n본문\n")+"\n본문\n")
}

// readGraph runs the graph view from the cycle's repository.
func (fixture *viewGraphFixture) readGraph() issueopsviewcontract.Result {
	fixture.t.Helper()
	var view issueopsviewcontract.Result
	if out := fixture.run("view", "--id", fixture.id, "--view", "graph", "--cwd", fixture.repo, "--json"); json.Unmarshal([]byte(out), &view) != nil || !view.OK || view.Graph == nil {
		fixture.t.Fatalf("view --view graph --json = %+v\n%s", view, out)
	}
	return view
}

// readNextAndGraph runs next and the graph view against the same state.
func (fixture *viewGraphFixture) readNextAndGraph() (issueopsnextcontract.Result, issueopsviewcontract.Result) {
	fixture.t.Helper()
	var next issueopsnextcontract.Result
	if out := fixture.run("next", "--id", fixture.id, "--cwd", fixture.repo, "--json"); json.Unmarshal([]byte(out), &next) != nil {
		fixture.t.Fatalf("next --json is not JSON:\n%s", out)
	}
	return next, fixture.readGraph()
}

func viewGraphStaleGates(graph *issueopsviewcontract.Graph) []string {
	gates := []string{}
	for _, edge := range graph.Edges {
		if edge.State == issueopsviewcontract.EdgeStateStale {
			gates = append(gates, edge.Gate)
		}
	}
	slices.Sort(gates)
	return gates
}

func viewGraphNextStaleKeys(missing []string) []string {
	keys := []string{}
	for _, key := range missing {
		if strings.HasSuffix(key, "_stale") {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return keys
}

// 프로덕션 composition의 `view --view graph`는 같은 cycle에서 프로덕션 `next`가 보고한
// `_stale` 키를 그대로 stale edge로 옮긴다. 승인 직후와 문서를 고친 뒤 두 상태를 읽는다.
func TestViewGraphUsesProductionNext(t *testing.T) {
	fixture := newViewGraphFixture(t)
	fixture.stagePlan("# plan")
	fixture.approveArchitecture("")

	// (a) approved architecture plus a staged plan: nothing is stale.
	next, view := fixture.readNextAndGraph()
	if next.Stage.Key != issueopsnextcontract.StagePlanReview || view.Graph.JudgedBy.Stage != next.Stage.Key {
		t.Fatalf("a: next stage = %q, judged_by.stage = %q, want plan.review for both", next.Stage.Key, view.Graph.JudgedBy.Stage)
	}
	if got := viewGraphNextStaleKeys(next.Missing); len(got) != 0 {
		t.Fatalf("a: next stale keys = %v, want none", got)
	}
	if got := viewGraphStaleGates(view.Graph); len(got) != 0 {
		t.Fatalf("a: graph stale gates = %v, want none", got)
	}
	notReported := 0
	for _, edge := range view.Graph.Edges {
		if edge.From == "architecture_doc" && edge.To == "architecture_review" && edge.State == issueopsviewcontract.EdgeStateNotReported {
			notReported++
		}
	}
	if notReported != 1 || view.Graph.UnknownReason != "" {
		t.Fatalf("a: architecture edge not_reported = %d, unknown_reason = %q\n%+v", notReported, view.Graph.UnknownReason, view.Graph.Edges)
	}

	// (b) editing the architecture after approval makes its review stale.
	fixture.stage("architecture", fixture.architecture+"추가한 절충점\n")
	next, view = fixture.readNextAndGraph()
	want := []string{"architecture_review_stale"}
	if got := viewGraphNextStaleKeys(next.Missing); !slices.Equal(got, want) {
		t.Fatalf("b: next stale keys = %v, want %v (stage %s)", got, want, next.Stage.Key)
	}
	if got := viewGraphStaleGates(view.Graph); !slices.Equal(got, want) {
		t.Fatalf("b: graph stale gates = %v, want %v", got, want)
	}
	if view.Graph.JudgedBy.Stage != next.Stage.Key || !slices.Equal(view.Graph.JudgedBy.StaleKeys, want) || len(view.Graph.JudgedBy.UnmappedStaleKeys) != 0 {
		t.Fatalf("b: judged_by = %+v, next stage = %q", view.Graph.JudgedBy, next.Stage.Key)
	}
}

// snapshotViewGraphBuckets는 state DB의 모든 bucket에서 (bucket, id, sha256(data))
// 줄을 정렬해 돌려준다. 읽기 전용으로 열어 snapshot 자체가 state를 바꾸지 않는다.
func snapshotViewGraphBuckets(t *testing.T, root string) []string {
	t.Helper()
	database, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(root, "issueops.db"))+"?mode=ro&_pragma=query_only(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	rows, err := database.Query(`SELECT bucket, id, data FROM records ORDER BY bucket, id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	lines := []string{}
	for rows.Next() {
		var bucket, id string
		var data []byte
		if err := rows.Scan(&bucket, &id, &data); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		lines = append(lines, bucket+"/"+id+" "+hex.EncodeToString(sum[:]))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return lines
}

func viewGraphStateFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	slices.Sort(names)
	return names
}

// graph 조망은 읽기 전용이다. 프로덕션 배선으로 `next`를 부르고 git을 읽지만 state의 모든
// row와 파일을 그대로 두고 원격 CLI(gh, glab)를 부르지 않는다.
func TestIssueOpsViewGraphIsReadOnly(t *testing.T) {
	fixture := newViewGraphFixture(t)
	fixture.stagePlan("# plan")
	fixture.approveArchitecture("")

	root := issueOpsStateRoot()
	retained, err := json.Marshal(issueopscontract.CycleOutcome{
		SchemaVersion: 1, ID: "io-gone", Repo: fixture.repo, Outcome: "abandoned", RetainedAt: "2026-09-09T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	database, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Apply(context.Background(), []port.RecordMutation{
		{Bucket: "issueops_cycle_outcome_v1", ID: "io-gone", Data: retained},
	}); err != nil {
		t.Fatal(err)
	}

	buckets := snapshotViewGraphBuckets(t, root)
	files := viewGraphStateFiles(t, root)
	for _, prefix := range []string{"issueops_v1/" + fixture.id + " ", "issueops_cycle_outcome_v1/io-gone "} {
		if !slices.ContainsFunc(buckets, func(line string) bool { return strings.HasPrefix(line, prefix) }) {
			t.Fatalf("the state lacks a row starting %q:\n%v", prefix, buckets)
		}
	}

	markers, bin := t.TempDir(), t.TempDir()
	for _, tool := range []string{"gh", "glab"} {
		marker := filepath.ToSlash(filepath.Join(markers, tool+".called"))
		testsupport.WriteFakeCommand(t, bin, tool, "#!/bin/sh\n: > '"+marker+"'\nexit 1\n")
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	view := fixture.readGraph()
	judged := view.Graph.JudgedBy.Stage
	if judged != issueopsnextcontract.StagePlanReview && judged != issueopsnextcontract.StagePlanHandoff {
		t.Fatalf("judged_by.stage = %q, want plan.review or plan.handoff", judged)
	}
	notReported := 0
	for _, edge := range view.Graph.Edges {
		if edge.Binding != "" && edge.State == issueopsviewcontract.EdgeStateNotReported {
			notReported++
		}
	}
	if notReported == 0 {
		t.Fatalf("the graph has no not_reported binding edge, so it did not judge: %+v", view.Graph)
	}

	if after := snapshotViewGraphBuckets(t, root); !slices.Equal(after, buckets) {
		t.Fatalf("view --view graph changed the state records:\nbefore=%v\nafter=%v", buckets, after)
	}
	if after := viewGraphStateFiles(t, root); !slices.Equal(after, files) {
		t.Fatalf("view --view graph changed the state files:\nbefore=%v\nafter=%v", files, after)
	}
	for _, tool := range []string{"gh", "glab"} {
		if _, err := os.Stat(filepath.Join(markers, tool+".called")); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("view --view graph invoked %s (stat err=%v)", tool, err)
		}
	}
}

// 프로덕션 composition의 WorktreePresent는 next가 쓰는 git 관측이다. execution의 worktree가
// 실제 git worktree였다가 디렉터리가 사라지면 fingerprint·head edge는 unknown이고
// unknown_reason은 worktree_missing이다. digest edge는 worktree와 무관해 계속 판정한다.
// worktree가 있을 때는 같은 record에서 아무 edge도 unknown이 아니어야 한다(양성 대조).
func TestViewGraphProductionWorktreeMissing(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	for _, name := range []string{"CLAUDE_CODE_SESSION_ID", "CODEX_THREAD_ID", "PI_SESSION_ID", "ISSUEOPS_OMP_SESSION_ID", "OPENCODE_SESSION_ID"} {
		t.Setenv(name, "")
	}
	const session = "view-graph-worktree-session"
	t.Setenv("CODEX_THREAD_ID", session)

	repo := makeGitRepoForContract(t)
	head := strings.TrimSpace(gitOutputForViewGraph(t, repo, "rev-parse", "HEAD"))
	worktree := filepath.Join(t.TempDir(), "wt")
	const branch = "1-view-graph"
	gitOutputForViewGraph(t, repo, "worktree", "add", "-q", "-b", branch, worktree, head)

	receipt, err := issueopscore.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	id := "io-view-graph-worktree"
	data, err := issueopsrecord.Encode(issueopscontract.Record{
		SchemaVersion: issueopscontract.SchemaVersion,
		ID:            id, Repo: repo, Branch: branch, Phase: issueopscontract.PhasePlan,
		CreatedAt: "2026-11-01T00:00:00Z", UpdatedAt: "2026-11-01T00:00:00Z",
		IssueURL:             "https://github.com/example/repo/issues/1",
		BranchPrepare:        &issueopscontract.BranchPrepare{Provider: "github", BaseSHA: head, LinkVerified: true},
		ArchitectureReview:   &reviewcontract.ArchitectureReview{Approved: true, ReviewedArchitectureDigest: "sha256:architecture", ReviewedAt: "2026-11-01T00:00:00Z"},
		ImplementationReview: &issueopscontract.ImplementationReview{Verdict: "pass", RecordedAt: "2026-11-01T00:00:00Z"},
		Execution: &issueopscontract.Execution{
			Workspace: issueopscontract.Workspace{SourceRoot: repo, Root: worktree, Branch: branch, BaseHead: head, LinkedAt: "2026-11-01T00:00:00Z"},
			Lease: issueopscontract.WriteLease{
				Generation: 1, Status: issueopscontract.LeaseStatusActive, ClaimedAt: "2026-11-01T00:00:00Z",
				Holder: &issueopscontract.NativeActor{Host: "codex", SessionID: session, SessionProcess: &receipt},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	database, err := sqlstore.Open(issueOpsStateRoot())
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Apply(context.Background(), []port.RecordMutation{
		{Bucket: "issueops_v" + strconv.Itoa(issueopscontract.SchemaVersion), ID: id, Data: data},
	}); err != nil {
		t.Fatal(err)
	}

	read := func() *issueopsviewcontract.Graph {
		t.Helper()
		var view issueopsviewcontract.Result
		out := captureStdoutForContract(t, func() error {
			return runIssueOps([]string{"view", "--id", id, "--view", "graph", "--cwd", repo, "--json"})
		})
		if json.Unmarshal([]byte(out), &view) != nil || !view.OK || view.Graph == nil {
			t.Fatalf("view --view graph --json = %+v\n%s", view, out)
		}
		return view.Graph
	}
	states := func(graph *issueopsviewcontract.Graph) map[string]string {
		found := map[string]string{}
		for _, edge := range graph.Edges {
			if edge.Binding != "" {
				found[edge.Gate] = edge.State
			}
		}
		return found
	}

	present := read()
	if present.UnknownReason != "" {
		t.Fatalf("with the worktree present: unknown_reason = %q (judged_by %+v), want none", present.UnknownReason, present.JudgedBy)
	}
	wantPresent := map[string]string{
		"architecture_review_stale":   issueopsviewcontract.EdgeStateNotReported,
		"implementation_review_stale": issueopsviewcontract.EdgeStateNotReported,
	}
	if got := states(present); !maps.Equal(got, wantPresent) {
		t.Fatalf("with the worktree present: edge states = %v, want %v", got, wantPresent)
	}

	if err := os.RemoveAll(worktree); err != nil {
		t.Fatal(err)
	}
	missing := read()
	if missing.UnknownReason != issueopsviewcontract.UnknownWorktreeMissing {
		t.Fatalf("without the worktree: unknown_reason = %q (judged_by %+v), want worktree_missing", missing.UnknownReason, missing.JudgedBy)
	}
	wantMissing := map[string]string{
		"architecture_review_stale":   issueopsviewcontract.EdgeStateNotReported,
		"implementation_review_stale": issueopsviewcontract.EdgeStateUnknown,
	}
	if got := states(missing); !maps.Equal(got, wantMissing) {
		t.Fatalf("without the worktree: edge states = %v, want %v", got, wantMissing)
	}
}
