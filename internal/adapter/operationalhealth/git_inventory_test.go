package operationalhealth

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	operationalhealthcontract "issueops/internal/contract/operationalhealth"
	corehealth "issueops/internal/domain/operationalhealth"
	"issueops/internal/testsupport"
)

var (
	oidA = strings.Repeat("a", 40)
	oidB = strings.Repeat("b", 40)
)

// scriptedGit answers Git inventory calls by the first argument pair.
type scriptedGit struct {
	outputs map[string]string
	errs    map[string]error
	calls   []string
}

func (git *scriptedGit) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	key := strings.Join(args, " ")
	git.calls = append(git.calls, key)
	if err := git.errs[args[0]]; err != nil {
		return nil, err
	}
	return []byte(git.outputs[args[0]]), nil
}

func problemCodes(problems []operationalhealthcontract.InventoryProblem) []string {
	var codes []string
	for _, problem := range problems {
		codes = append(codes, problem.Code)
	}
	return codes
}

func TestValidOIDAcceptsOnlyLowercaseHexOfSha1OrSha256Length(t *testing.T) {
	cases := map[string]bool{
		oidA:                           true,
		strings.Repeat("0f", 32):       true,
		strings.Repeat("a", 39):        false,
		strings.Repeat("a", 41):        false,
		strings.Repeat("A", 40):        false,
		strings.Repeat("g", 40):        false,
		strings.Repeat("a", 39) + "-":  false,
		"":                             false,
		strings.Repeat("a", 63) + "\n": false,
	}
	for value, want := range cases {
		if got := validOID(value); got != want {
			t.Errorf("validOID(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestParseWorktreesMarksCanonicalAndRejectsMalformedRecords(t *testing.T) {
	repo := canonicalInventoryPath(t.TempDir())
	other := filepath.Join(repo, "wt")
	output := "worktree " + repo + "\x00HEAD " + oidA + "\x00branch refs/heads/main\x00\x00" +
		"worktree " + other + "\x00HEAD " + oidB + "\x00detached\x00\x00"
	got, err := parseWorktrees([]byte(output), repo)
	if err != nil {
		t.Fatal(err)
	}
	want := []operationalhealthcontract.GitWorktree{
		{Path: repo, Head: oidA, Branch: "main", Canonical: true},
		{Path: canonicalInventoryPath(other), Head: oidB},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}

	for name, bad := range map[string]string{
		"missing path":      "HEAD " + oidA + "\x00branch refs/heads/x\x00\x00",
		"missing head":      "worktree " + repo + "\x00\x00",
		"malformed head":    "worktree " + repo + "\x00HEAD nothex\x00\x00",
		"unterminated tail": "worktree " + repo + "\x00HEAD short",
	} {
		if _, err := parseWorktrees([]byte(bad), repo); err == nil {
			t.Errorf("%s: accepted malformed output", name)
		}
	}
	if got, err := parseWorktrees(nil, repo); err != nil || len(got) != 0 {
		t.Fatalf("empty output must be valid and empty: %v %v", got, err)
	}
}

func TestParseLocalRefsRequiresCompleteHeadsTuples(t *testing.T) {
	got, err := parseLocalRefs([]byte("refs/heads/main\x00" + oidA + "\x00\nrefs/heads/feat/x\x00" + oidB + "\x00\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []operationalhealthcontract.GitRef{
		{Name: "refs/heads/main", Branch: "main", OID: oidA, Location: "local"},
		{Name: "refs/heads/feat/x", Branch: "feat/x", OID: oidB, Location: "local"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
	for name, bad := range map[string]string{
		"odd field count": "refs/heads/main\x00" + oidA + "\x00refs/heads/x\x00",
		"non-heads ref":   "refs/tags/v1\x00" + oidA + "\x00",
		"bad oid":         "refs/heads/main\x00zzz\x00",
	} {
		if _, err := parseLocalRefs([]byte(bad)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestParseRemoteRefsRequiresOIDAndHeadsRefPerLine(t *testing.T) {
	got, err := parseRemoteRefs([]byte(oidA + "\trefs/heads/main\n\n  " + oidB + " refs/heads/dev  \n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []operationalhealthcontract.GitRef{
		{Name: "refs/heads/main", Branch: "main", OID: oidA, Location: "remote"},
		{Name: "refs/heads/dev", Branch: "dev", OID: oidB, Location: "remote"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
	for name, bad := range map[string]string{
		"extra field": oidA + " refs/heads/a extra\n",
		"short line":  oidA + "\n",
		"tag ref":     oidA + " refs/tags/v1\n",
		"bad oid":     "xyz refs/heads/a\n",
	} {
		if _, err := parseRemoteRefs([]byte(bad)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestCollectGitRecordsProblemPerFailedOrMalformedCommand(t *testing.T) {
	repo := canonicalInventoryPath(t.TempDir())
	git := &scriptedGit{
		outputs: map[string]string{
			"symbolic-ref": "  \n",
			"rev-parse":    "not-an-oid\n",
			"status":       " M file\x00",
			"worktree":     "worktree " + repo + "\x00HEAD " + oidA + "\x00\x00",
			"for-each-ref": "refs/heads/main\x00",
		},
		errs: map[string]error{"ls-remote": errors.New("network down")},
	}
	snapshot := corehealth.Snapshot{RepoRoot: repo}
	Collector{Git: git}.collectGit(t.Context(), &snapshot, true)

	want := []string{"git_symbolic_ref_failed", "git_head_failed", "git_local_refs_failed", "git_remote_refs_failed", "git_source_identity_mismatch"}
	// the rejected head text is still retained, so the canonical worktree head disagrees with it.
	if got := problemCodes(snapshot.InventoryProblems); !reflect.DeepEqual(got, want) {
		t.Fatalf("problems = %v, want %v", got, want)
	}
	if snapshot.SourceClean {
		t.Fatal("porcelain output means the source is dirty")
	}
	if len(snapshot.GitWorktrees) != 1 || !snapshot.GitWorktrees[0].Canonical || snapshot.GitWorktrees[0].Clean {
		t.Fatalf("canonical worktree must inherit dirty state: %+v", snapshot.GitWorktrees)
	}
}

func TestCollectGitFlagsCanonicalWorktreeHeadThatDiffersFromSourceHead(t *testing.T) {
	repo := canonicalInventoryPath(t.TempDir())
	git := &scriptedGit{outputs: map[string]string{
		"symbolic-ref": "main\n",
		"rev-parse":    oidA + "\n",
		"status":       "",
		"worktree":     "worktree " + repo + "\x00HEAD " + oidB + "\x00branch refs/heads/main\x00\x00",
		"for-each-ref": "",
		"ls-remote":    "",
	}}
	snapshot := corehealth.Snapshot{RepoRoot: repo}
	Collector{Git: git}.collectGit(t.Context(), &snapshot, true)
	if got := problemCodes(snapshot.InventoryProblems); !reflect.DeepEqual(got, []string{"git_source_identity_mismatch"}) {
		t.Fatalf("problems = %v", got)
	}
	if snapshot.CanonicalBranch != "main" || snapshot.SourceHead != oidA || !snapshot.SourceClean || !snapshot.GitWorktrees[0].Clean {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

func TestCollectGitWithoutRunnerReportsMissingRunner(t *testing.T) {
	snapshot := corehealth.Snapshot{RepoRoot: "/repo"}
	Collector{}.collectGit(t.Context(), &snapshot, true)
	if got := problemCodes(snapshot.InventoryProblems); !reflect.DeepEqual(got, []string{"git_runner_missing"}) {
		t.Fatalf("problems = %v", got)
	}
}

func TestCollectLocalSkipsRemoteRefs(t *testing.T) {
	repo := t.TempDir()
	git := &scriptedGit{outputs: map[string]string{
		"symbolic-ref": "main\n",
		"rev-parse":    oidA + "\n",
		"worktree":     "",
		"for-each-ref": "refs/heads/main\x00" + oidA + "\x00",
	}}
	snapshot := Collector{IssueOps: emptyIssueOpsReader(t), Git: git}.CollectLocal(t.Context(), repo)
	for _, call := range git.calls {
		if strings.HasPrefix(call, "ls-remote") {
			t.Fatalf("CollectLocal must not contact the remote: %v", git.calls)
		}
	}
	if snapshot.RepoRoot != canonicalInventoryPath(repo) || snapshot.SourceHead != oidA || len(snapshot.LocalRefs) != 1 || len(snapshot.RemoteRefs) != 0 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if len(snapshot.InventoryProblems) != 0 {
		t.Fatalf("problems = %v", snapshot.InventoryProblems)
	}
}

func TestCollectRejectsEmptyRepoWithoutTouchingGit(t *testing.T) {
	git := &scriptedGit{}
	snapshot := Collector{IssueOps: emptyIssueOpsReader(t), Git: git}.Collect(t.Context(), "   ")
	if got := problemCodes(snapshot.InventoryProblems); !reflect.DeepEqual(got, []string{"repo_invalid"}) {
		t.Fatalf("problems = %v", got)
	}
	if len(git.calls) != 0 {
		t.Fatalf("git calls=%v", git.calls)
	}
}

func TestCollectSortsGitInventoryProblemsBySource(t *testing.T) {
	git := &scriptedGit{errs: map[string]error{
		"symbolic-ref": errors.New("x"), "rev-parse": errors.New("x"), "status": errors.New("x"),
		"worktree": errors.New("x"), "for-each-ref": errors.New("x"), "ls-remote": errors.New("x"),
	}}
	snapshot := Collector{IssueOps: emptyIssueOpsReader(t), Git: git}.Collect(t.Context(), t.TempDir())
	codes := problemCodes(snapshot.InventoryProblems)
	if len(codes) != 6 {
		t.Fatalf("problems = %v", codes)
	}
	sources := make([]string, len(snapshot.InventoryProblems))
	for i, problem := range snapshot.InventoryProblems {
		sources[i] = problem.Source
	}
	if !sortedStrings(sources) {
		t.Fatalf("problems not sorted by source: %v", sources)
	}
}

func sortedStrings(values []string) bool {
	for i := 1; i < len(values); i++ {
		if values[i-1] > values[i] {
			return false
		}
	}
	return true
}

func TestSortSnapshotOrdersEveryCollectionDeterministically(t *testing.T) {
	snapshot := corehealth.Snapshot{
		InventoryProblems: []operationalhealthcontract.InventoryProblem{{Source: "b"}, {Source: "a", Code: "z"}, {Source: "a", Code: "y"}},
		GitWorktrees:      []operationalhealthcontract.GitWorktree{{Path: "/b"}, {Path: "/a", Branch: "x"}, {Path: "/a", Branch: "w"}},
		LocalRefs:         []operationalhealthcontract.GitRef{{Name: "refs/heads/b"}, {Name: "refs/heads/a"}},
		RemoteRefs:        []operationalhealthcontract.GitRef{{Name: "r2"}, {Name: "r1"}},
		StateArtifacts:    []operationalhealthcontract.StateArtifact{{Path: "/z"}, {Path: "/a"}},
	}
	sortSnapshot(&snapshot)

	if got := problemCodes(snapshot.InventoryProblems); !reflect.DeepEqual(got, []string{"y", "z", ""}) {
		t.Errorf("problems = %v", got)
	}
	if snapshot.GitWorktrees[0].Branch != "w" || snapshot.GitWorktrees[2].Path != "/b" {
		t.Errorf("worktrees = %+v", snapshot.GitWorktrees)
	}
	if snapshot.LocalRefs[0].Name != "refs/heads/a" || snapshot.RemoteRefs[0].Name != "r1" {
		t.Errorf("refs = %+v %+v", snapshot.LocalRefs, snapshot.RemoteRefs)
	}
	if snapshot.StateArtifacts[0].Path != "/a" {
		t.Errorf("state artifacts unsorted: %+v", snapshot.StateArtifacts)
	}
}

func TestOrderedBeforeComparesFirstDifferingFieldThenLength(t *testing.T) {
	if !orderedBefore([]string{"a", "b"}, []string{"a", "c"}) || orderedBefore([]string{"a", "c"}, []string{"a", "b"}) {
		t.Fatal("first differing field decides")
	}
	if !orderedBefore([]string{"a"}, []string{"a", "b"}) || orderedBefore([]string{"a", "b"}, []string{"a"}) {
		t.Fatal("shorter prefix sorts first")
	}
	if orderedBefore([]string{"a"}, []string{"a"}) {
		t.Fatal("equal keys are not strictly ordered")
	}
}

func TestExecGitRunnerRequiresArgsAndWrapsCancellation(t *testing.T) {
	if _, err := (ExecGitRunner{}).Run(t.Context(), t.TempDir()); err == nil || !strings.Contains(err.Error(), "git arguments are required") {
		t.Fatalf("err = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := (ExecGitRunner{}).Run(ctx, t.TempDir(), "--version")
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "git inventory command") {
		t.Fatalf("canceled context must surface as wrapped cancellation: %v", err)
	}
}

func TestExecGitRunnerReadsDirtyRepositoryState(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	testsupport.IsolateGitConfig(t)
	repo := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "init"}} {
		command := exec.Command("git", args...)
		command.Dir = repo
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runner := ExecGitRunner{timeout: 10 * time.Second}
	branch, err := runner.Run(t.Context(), repo, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || strings.TrimSpace(string(branch)) != "main" {
		t.Fatalf("branch=%q err=%v", branch, err)
	}
	if err := os.WriteFile(filepath.Join(repo, "dirty"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot := Collector{IssueOps: emptyIssueOpsReader(t), Git: runner}.CollectLocal(t.Context(), repo)
	if snapshot.CanonicalBranch != "main" || !validOID(snapshot.SourceHead) || snapshot.SourceClean {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if len(snapshot.GitWorktrees) != 1 || !snapshot.GitWorktrees[0].Canonical || len(snapshot.LocalRefs) != 1 || len(snapshot.InventoryProblems) != 0 {
		t.Fatalf("worktrees=%+v refs=%+v problems=%+v", snapshot.GitWorktrees, snapshot.LocalRefs, snapshot.InventoryProblems)
	}
}

func TestCanonicalInventoryPathResolvesSymlinksAndMissingTails(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	if got := canonicalInventoryPath("  "); got != "" {
		t.Fatalf("blank path = %q", got)
	}
	if got := canonicalInventoryPath(link + "/"); got != root {
		t.Fatalf("symlink = %q, want %q", got, root)
	}
	if got, want := canonicalInventoryPath(filepath.Join(link, "missing", "deeper")), filepath.Join(root, "missing", "deeper"); got != want {
		t.Fatalf("missing tail under symlink = %q, want %q", got, want)
	}
	t.Chdir(root)
	if got := canonicalInventoryPath("rel/../link"); got != root {
		t.Fatalf("relative path = %q, want %q", got, root)
	}
}

func TestExecGitRunnerAppliesDefaultTimeoutWhenUnset(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	testsupport.IsolateGitConfig(t)
	for name, runner := range map[string]ExecGitRunner{"zero": {}, "negative": {timeout: -time.Second}} {
		output, err := runner.Run(t.Context(), t.TempDir(), "--version")
		if err != nil || !strings.HasPrefix(string(output), "git version") {
			t.Fatalf("%s timeout: output=%q err=%v", name, output, err)
		}
	}
}
