package issueops

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"issueops/internal/adapter/issueops/implementation"
	preflightadapter "issueops/internal/adapter/preflight"
	"issueops/internal/contract/issueops"
)

// local readiness는 단계 분류처럼 자주 불리므로 원격을 때리면 안 된다.
func TestIssueOpsLocalPRReadinessNeverFetches(t *testing.T) {
	var commands []string
	restore := stubIssueOpsGit(t, &commands)
	defer restore()

	record := issueops.IssueOpsRecord{
		ID: "io-local", Repo: t.TempDir(), Branch: "12-local",
		Phase: issueops.IssueOpsPhaseAISlopClean,
	}
	ready := IssueOpsLocalPRReadiness(record)

	for _, command := range commands {
		if strings.HasPrefix(command, "fetch") {
			t.Fatalf("local readiness must not fetch, ran %q", command)
		}
	}
	for _, key := range ready.Missing {
		if key == "upstream_fetch" || key == "upstream_synced" {
			t.Fatalf("local readiness must not judge upstream sync, got %v", ready.Missing)
		}
	}
	if ready.Strict {
		t.Fatal("local readiness is not the strict surface")
	}
}

// strict는 같은 관측에 fetch와 동기화 판정을 더한 것이다.
func TestIssueOpsStrictPRReadinessStillFetches(t *testing.T) {
	var commands []string
	restore := stubIssueOpsGit(t, &commands)
	defer restore()

	record := issueops.IssueOpsRecord{
		ID: "io-strict", Repo: t.TempDir(), Branch: "12-strict",
		Phase: issueops.IssueOpsPhaseAISlopClean,
	}
	ready := IssueOpsStrictPRReadiness(record)
	if !ready.Strict {
		t.Fatal("strict readiness must report itself as strict")
	}
	fetched := false
	for _, command := range commands {
		if strings.HasPrefix(command, "fetch") {
			fetched = true
		}
	}
	if !fetched {
		t.Fatalf("strict readiness must fetch, ran %v", commands)
	}
}

func TestIssueOpsStrictPRReadinessReobservesSchemaPathsAfterFetch(t *testing.T) {
	repo := initIssueOpsRepo(t)
	branch := "12-post-fetch-schema"
	for _, args := range [][]string{
		{"checkout", "-q", "-b", branch},
		{"add", "feature.go"},
		{"commit", "-q", "-m", "feat: add non-schema change"},
		{"push", "-q", "-u", "origin", branch},
	} {
		if args[0] == "add" {
			writeRepoFileForTest(t, repo, "feature.go", "package feature\n")
		}
		if code, _, stderr := preflightadapter.GitCmd(repo, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}

	remote := strings.TrimSpace(preflightadapter.GitOut(repo, "remote", "get-url", "origin"))
	cloneRoot := t.TempDir()
	upstream := filepath.Join(cloneRoot, "upstream")
	if code, _, stderr := preflightadapter.GitCmd(cloneRoot, "clone", "-q", "--branch", "main", remote, upstream); code != 0 {
		t.Fatalf("clone upstream fixture: %s", stderr)
	}
	for _, args := range [][]string{
		{"config", "user.name", "IssueOps Upstream"},
		{"config", "user.email", "upstream@example.test"},
		{"add", "db/migrations/001_remote.sql"},
		{"commit", "-q", "-m", "feat: advance base with schema change"},
		{"push", "-q", "origin", "main"},
	} {
		if args[0] == "add" {
			writeRepoFileForTest(t, upstream, "db/migrations/001_remote.sql", "CREATE TABLE remote_change(id bigint);\n")
		}
		if code, _, stderr := preflightadapter.GitCmd(upstream, args...); code != 0 {
			t.Fatalf("upstream git %v failed: %s", args, stderr)
		}
	}

	record := baseAdvancedRecord(repo, "main")
	record.ID = "io-post-fetch-schema"
	record.Branch = branch
	record.BranchPrepare.Branch = branch
	record.BranchPrepare.BaseSHA = strings.Repeat("f", 40)
	record.Execution = &issueops.Execution{Mode: issueops.ExecutionModeDirect}
	before := implementation.ObserveLocalChangesAt(record, repo)
	if !before.Verified || !reflect.DeepEqual(before.Paths, []string{"feature.go"}) || before.Fingerprint == "" {
		t.Fatalf("pre-fetch fallback observation = %+v", before)
	}
	record.AISlopCleanFingerprint = before.Fingerprint

	ready := IssueOpsStrictPRReadiness(record)

	if ready.CurrentFingerprint != before.Fingerprint {
		t.Fatalf("strict fingerprint must stay bound to the preflight snapshot: got %q want %q", ready.CurrentFingerprint, before.Fingerprint)
	}
	if !containsString(ready.Missing, "schema_evidence") {
		t.Fatalf("post-fetch schema path must activate schema evidence: missing=%v warnings=%v", ready.Missing, ready.Warnings)
	}
	for _, unexpected := range []string{"current_fingerprint", "ai_slop_clean_fingerprint", "ai_slop_clean_stale", "upstream_fetch", "upstream_synced"} {
		if containsString(ready.Missing, unexpected) {
			t.Fatalf("strict readiness added %q: missing=%v warnings=%v", unexpected, ready.Missing, ready.Warnings)
		}
	}
	if hasBaseAdvancedWarning(ready) {
		t.Fatalf("base warning must preserve its pre-fetch ordering: %v", ready.Warnings)
	}
	after := implementation.ChangedPaths(record)
	if !reflect.DeepEqual(after, []string{"db/migrations/001_remote.sql", "feature.go"}) {
		t.Fatalf("post-fetch fallback paths = %v", after)
	}
}

func TestIssueOpsLocalPRReadinessSharesOneVerifiedChangeObservationWithSchemaGate(t *testing.T) {
	repo := gitRepoWithProjectDocsForTest(t)
	writeRepoFileForTest(t, repo, "db/migrations/001_add_index.sql", "CREATE INDEX idx_x ON x(id);\n")
	baseSHA := strings.TrimSpace(preflightadapter.GitOut(repo, "rev-parse", "HEAD"))
	branch := strings.TrimSpace(preflightadapter.GitOut(repo, "branch", "--show-current"))
	record := issueops.IssueOpsRecord{
		ID: "io-local-observation", Repo: repo, WorktreePath: repo, Branch: branch,
		Phase: issueops.IssueOpsPhaseAISlopClean, PlanPath: filepath.Join(repo, ".issueops", "ADR.md"),
		BranchPrepare: &issueops.IssueOpsBranchPrepare{BaseBranch: branch, BaseSHA: baseSHA, LinkVerified: true},
		Execution:     &issueops.Execution{Mode: issueops.ExecutionModeDirect},
		AISlopCleanAt: "2026-01-01T00:00:00Z",
	}
	previousCmd, previousRaw := implementation.GitCmd, implementation.GitCmdRaw
	var commands []string
	implementation.GitCmd = func(dir string, args ...string) (int, string, string) {
		commands = append(commands, strings.Join(args, " "))
		return preflightadapter.GitCmd(dir, args...)
	}
	implementation.GitCmdRaw = func(dir string, args ...string) (int, string, string) {
		commands = append(commands, strings.Join(args, " "))
		return preflightadapter.GitCmdRaw(dir, args...)
	}
	t.Cleanup(func() { implementation.GitCmd, implementation.GitCmdRaw = previousCmd, previousRaw })

	ready := IssueOpsLocalPRReadiness(record)

	if !containsString(ready.Missing, "schema_evidence") {
		t.Fatalf("schema change must keep the schema evidence gate: %v", ready.Missing)
	}
	counts := map[string]int{}
	for _, command := range commands {
		switch {
		case strings.HasPrefix(command, "rev-parse --verify"):
			counts["base"]++
		case strings.HasPrefix(command, "diff --name-only"):
			counts["diff"]++
		case strings.HasPrefix(command, "status --porcelain"):
			counts["status"]++
		case strings.HasPrefix(command, "rev-parse --is-inside-work-tree"):
			counts["root"]++
		}
	}
	if counts["root"] != 0 || counts["base"] != 1 || counts["diff"] != 2 || counts["status"] != 2 || len(commands) != 5 {
		t.Fatalf("readiness change observation commands = %v (counts=%v), want one base resolution and two verified snapshots", commands, counts)
	}
}

func TestObserveIssueOpsLocalPRReadinessReturnsTheReadinessChangeSet(t *testing.T) {
	repo := gitRepoWithProjectDocsForTest(t)
	writeRepoFileForTest(t, repo, "db/migrations/001_add_index.sql", "CREATE INDEX idx_x ON x(id);\n")
	record := issueops.IssueOpsRecord{
		ID: "io-local-result", Repo: repo, WorktreePath: repo,
		BranchPrepare: &issueops.IssueOpsBranchPrepare{
			BaseBranch: strings.TrimSpace(preflightadapter.GitOut(repo, "branch", "--show-current")),
			BaseSHA:    strings.TrimSpace(preflightadapter.GitOut(repo, "rev-parse", "HEAD")),
		},
		Execution: &issueops.Execution{Mode: issueops.ExecutionModeDirect},
	}

	ready, observation := ObserveIssueOpsLocalPRReadiness(record)

	if ready.Strict {
		t.Fatal("the observation surface must retain local readiness semantics")
	}
	if !observation.Verified {
		t.Fatalf("stable local changes must be verified: %+v", observation)
	}
	wantPaths := []string{".issueops/CAUTIONS.md", "change.go", "db/migrations/001_add_index.sql"}
	if !reflect.DeepEqual(observation.Paths, wantPaths) {
		t.Fatalf("observed paths = %v, want %v", observation.Paths, wantPaths)
	}
	if ready.CurrentFingerprint == "" || ready.CurrentFingerprint != observation.Fingerprint {
		t.Fatalf("readiness and observation fingerprints diverged: ready=%q observation=%q", ready.CurrentFingerprint, observation.Fingerprint)
	}
}

// stubIssueOpsGit은 git 관측을 기록만 하는 스텁으로 바꾼다. upstream이 있는
// 것처럼 보여야 fetch 경로에 들어간다.
func stubIssueOpsGit(t *testing.T, commands *[]string) func() {
	t.Helper()
	previousCmd, previousOut := GitCmd, GitOut
	GitCmd = func(dir string, args ...string) (int, string, string) {
		*commands = append(*commands, strings.Join(args, " "))
		switch {
		case len(args) > 0 && args[0] == "rev-parse" && len(args) > 1 && args[1] == "--is-inside-work-tree":
			return 0, "true", ""
		default:
			return 0, "", ""
		}
	}
	GitOut = func(dir string, args ...string) string {
		*commands = append(*commands, strings.Join(args, " "))
		joined := strings.Join(args, " ")
		switch {
		case strings.HasPrefix(joined, "rev-parse --abbrev-ref --symbolic-full-name"):
			return "origin/12-local"
		case joined == "rev-list --left-right --count HEAD...@{u}":
			return "0\t0"
		default:
			return ""
		}
	}
	return func() { GitCmd, GitOut = previousCmd, previousOut }
}
