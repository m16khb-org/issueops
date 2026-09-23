package issueops

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/contract/issueops"

	_ "modernc.org/sqlite"
)

// span은 state root 전역이다(ADR 2026-07-07). 그래서 span 안에서 네트워크를 쓰는
// 호출이 멈추면 같은 root의 다른 모든 사이클 쓰기가 60초 상한까지 기다린다.
// 이 파일은 PATH 앞에 가짜 git을 둔다. 가짜 git은 호출된 순간 span lock
// 데이터베이스를 별도 connection으로 잡아 보고, 잡히지 않으면 그 호출이 span
// 안에서 일어났다고 기록한 뒤 진짜 git을 실행한다.
const (
	fakeGitModeEnv     = "ISSUEOPS_TEST_FAKE_GIT"
	fakeGitRealEnv     = "ISSUEOPS_TEST_REAL_GIT"
	fakeGitLockDBEnv   = "ISSUEOPS_TEST_SPAN_LOCK_DB"
	fakeGitLogEnv      = "ISSUEOPS_TEST_GIT_SPAN_LOG"
	fakeGitProbeAllEnv = "ISSUEOPS_TEST_GIT_PROBE_ALL"
)

var networkGitSubcommands = map[string]bool{
	"fetch": true, "ls-remote": true, "push": true, "pull": true, "clone": true,
}

func TestMain(m *testing.M) {
	if os.Getenv(fakeGitModeEnv) == "1" {
		os.Exit(runFakeGit(os.Args[1:]))
	}
	os.Exit(m.Run())
}

func runFakeGit(args []string) int {
	subcommand := gitSubcommand(args)
	if logPath := os.Getenv(fakeGitLogEnv); logPath != "" && (networkGitSubcommands[subcommand] || os.Getenv(fakeGitProbeAllEnv) == "1") {
		line := fmt.Sprintf("%s %s\n", subcommand, spanLockState(os.Getenv(fakeGitLockDBEnv)))
		if file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
			_, _ = file.WriteString(line)
			_ = file.Close()
		}
	}
	cmd := exec.Command(os.Getenv(fakeGitRealEnv), args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		_, _ = fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func gitSubcommand(args []string) string {
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "-C" || arg == "-c":
			i++
		case strings.HasPrefix(arg, "-"):
		default:
			return arg
		}
	}
	return ""
}

// spanLockState는 span이 잡혀 있으면 "held", 비어 있으면 "free"를 돌려준다.
// sqlstore span은 lock 데이터베이스에 BEGIN IMMEDIATE를 쥔 채 유지되므로 다른
// connection의 BEGIN IMMEDIATE는 같은 프로세스 안에서도 SQLITE_BUSY로 실패한다.
func spanLockState(lockDB string) string {
	if strings.TrimSpace(lockDB) == "" {
		return "error:no-lock-db"
	}
	db, err := sql.Open("sqlite", "file:"+lockDB+"?_pragma=busy_timeout(0)&_txlock=immediate")
	if err != nil {
		return "error:" + err.Error()
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		if strings.Contains(err.Error(), "SQLITE_BUSY") || strings.Contains(err.Error(), "database is locked") {
			return "held"
		}
		return "error:" + err.Error()
	}
	_ = tx.Rollback()
	return "free"
}

// installSpanProbingGit은 이후 git 호출을 가짜 git으로 돌리고 기록 파일 경로를
// 돌려준다. fixture는 이 함수보다 먼저 만들어야 불필요한 기록이 섞이지 않는다.
func installSpanProbingGit(t *testing.T, stateRoot string, probeAll bool) string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not available")
	}
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	script := "#!/bin/sh\n" + fakeGitModeEnv + "=1 exec '" + testBinary + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "git-span.log")
	t.Setenv(fakeGitRealEnv, realGit)
	t.Setenv(fakeGitLockDBEnv, spanLockDBPath(t, stateRoot))
	t.Setenv(fakeGitLogEnv, log)
	if probeAll {
		t.Setenv(fakeGitProbeAllEnv, "1")
	} else {
		t.Setenv(fakeGitProbeAllEnv, "")
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

func spanLockDBPath(t *testing.T, stateRoot string) string {
	t.Helper()
	abs, err := filepath.Abs(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(abs, "issueops.lock.db")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("span lock database must exist before probing: %v", err)
	}
	return path
}

// probedGitCalls는 기록 파일을 "subcommand state" 목록으로 읽는다.
func probedGitCalls(t *testing.T, log string) []string {
	t.Helper()
	file, err := os.Open(log)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var calls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			calls = append(calls, line)
		}
	}
	return calls
}

func requireGitCallsOutsideSpan(t *testing.T, log string, wantSubcommand string) {
	t.Helper()
	calls := probedGitCalls(t, log)
	sawWanted := false
	for _, call := range calls {
		if !strings.HasSuffix(call, " free") {
			t.Fatalf("git ran while the span lock was not free: %q (all calls: %v)", call, calls)
		}
		if strings.HasPrefix(call, wantSubcommand+" ") {
			sawWanted = true
		}
	}
	if wantSubcommand != "" && !sawWanted {
		t.Fatalf("expected git %s to run; probed calls: %v", wantSubcommand, calls)
	}
}

func TestSpanLockStateDetectsAHeldSpan(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "997-probe"})
	if err != nil {
		t.Fatal(err)
	}
	lockDB := spanLockDBPath(t, stateRoot)
	if state := spanLockState(lockDB); state != "free" {
		t.Fatalf("no span is held yet, got %s", state)
	}
	var inside string
	if err := withIssueOpsLock(t.Context(), stateRoot, record.ID, func(context.Context) error {
		inside = spanLockState(lockDB)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if inside != "held" {
		t.Fatalf("the probe must see the held span, got %s", inside)
	}
}

// PR 진입은 upstream을 fetch해 동기화를 확인한다. fetch는 네트워크 호출이므로
// span 밖에서 끝나야 한다.
func TestPRPhaseEntryFetchesUpstreamOutsideTheSpan(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "998-pr-entry"})
	if err != nil {
		t.Fatal(err)
	}
	log := installSpanProbingGit(t, stateRoot, false)
	// 이 레코드는 PR readiness를 채우지 않았으므로 전이는 거부된다. 확인할 것은
	// 거부 여부가 아니라 fetch가 어디서 실행됐는가다.
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(IssueOpsPhasePR)); err == nil {
		t.Fatal("an unprepared record must not enter the pr phase")
	}
	requireGitCallsOutsideSpan(t, log, "fetch")
}

// 재타깃은 provider readback과 origin 관측(git ls-remote)을 요구한다. 둘 다 원격
// 호출이므로 span 밖에서 한 번 관측하고, span 안에서는 그 결과만 쓴다.
func TestBranchRetargetObservesRemoteOutsideTheSpan(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	if code, _, stderr := GitCmd(repo, "push", "-q", "origin", "main:refs/heads/2803-umbrella"); code != 0 {
		t.Fatalf("create the retarget branch on origin: %s", stderr)
	}
	record := retargetReadyRecord(t, stateRoot, repo)
	lockDB := spanLockDBPath(t, stateRoot)
	log := installSpanProbingGit(t, stateRoot, false)

	readbackState := ""
	updated, err := RetargetIssueOpsBranchWithActor(stateRoot, record.ID, issueops.IssueOpsBranchRetargetRequest{
		BaseBranch: "2803-umbrella", Reason: "child MR retargeted to the umbrella",
	}, IssueOpsActor{}, BranchRetargetDeps{
		ObserveArtifactTargetBranch: func(issueops.IssueOpsRemoteArtifactVerification) (string, error) {
			readbackState = spanLockState(lockDB)
			return "2803-umbrella", nil
		},
	})
	if err != nil {
		t.Fatalf("retarget: %v", err)
	}
	if readbackState != "free" {
		t.Fatalf("provider readback must run outside the span, got %s", readbackState)
	}
	requireGitCallsOutsideSpan(t, log, "ls-remote")
	if updated.BranchPrepare == nil || updated.BranchPrepare.BaseBranch != "2803-umbrella" || len(updated.BranchPrepare.Retargets) != 1 {
		t.Fatalf("retarget must still record the new base and its history: %+v", updated.BranchPrepare)
	}
}

// span 밖에서 관측한 결과는 관측 대상이 span 안에서도 같을 때만 쓴다. 그사이
// 원격 artifact가 바뀌면 옛 관측으로 기록하지 않고 다시 시도하게 한다.
func TestBranchRetargetRejectsAnArtifactThatChangedAfterObservation(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	if code, _, stderr := GitCmd(repo, "push", "-q", "origin", "main:refs/heads/2803-umbrella"); code != 0 {
		t.Fatalf("create the retarget branch on origin: %s", stderr)
	}
	record := retargetReadyRecord(t, stateRoot, repo)
	_, err := RetargetIssueOpsBranchWithActor(stateRoot, record.ID, issueops.IssueOpsBranchRetargetRequest{
		BaseBranch: "2803-umbrella", Reason: "child MR retargeted to the umbrella",
	}, IssueOpsActor{}, BranchRetargetDeps{
		ObserveArtifactTargetBranch: func(issueops.IssueOpsRemoteArtifactVerification) (string, error) {
			// 관측 도중 다른 쓰기가 artifact를 바꾼다.
			changed, readErr := ReadIssueOps(stateRoot, record.ID)
			if readErr != nil {
				return "", readErr
			}
			changed.RemoteArtifact.URL = "https://github.com/example/issueops/pull/9999"
			if _, writeErr := writeIssueOps(stateRoot, changed); writeErr != nil {
				return "", writeErr
			}
			return "2803-umbrella", nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "changed while") {
		t.Fatalf("a retarget observed against a different artifact must be refused: %v", err)
	}
	current, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if current.BranchPrepare.BaseBranch != "main" || len(current.BranchPrepare.Retargets) != 0 {
		t.Fatalf("a refused retarget must not move the base: %+v", current.BranchPrepare)
	}
}

// evidence recorder는 변경 집합 fingerprint를 계산하려고 git을 여러 번 호출한다.
// 로컬 읽기라 불변식 위반은 아니지만, 전역 span을 그동안 붙잡으면 다른 사이클이
// 모두 기다린다. 관측은 span 밖에서 끝내고 span 안에서는 전제만 다시 확인한다.
func TestEvidenceRecordersObserveTheChangeSetOutsideTheSpan(t *testing.T) {
	stateRoot := t.TempDir()
	fixture, holder := activeLeaseEvidenceFixture(t, stateRoot)
	if err := os.WriteFile(filepath.Join(fixture.worktree, "AGENTS.md"), []byte("# agents\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	log := installSpanProbingGit(t, stateRoot, true)

	if _, err := RecordIssueOpsImplementationReviewWithActor(stateRoot, fixture.record.ID, IssueOpsImplementationReviewRequest{
		Verdict: "pass", Findings: []string{"none"}, Evidence: []string{"go test ./..."},
	}, holder); err != nil {
		t.Fatalf("implementation review: %v", err)
	}
	if _, err := RecordIssueOpsSchemaEvidenceWithActor(stateRoot, fixture.record.ID, IssueOpsSchemaEvidenceRequest{
		Measurements: []string{"orders rows=1"}, Sources: []string{"psql"},
	}, holder); err != nil {
		t.Fatalf("schema evidence: %v", err)
	}
	if _, err := RecordIssueOpsProjectDocsReviewWithActor(stateRoot, fixture.record.ID, IssueOpsProjectDocsReviewRequest{
		Verdict: "no-change", ReviewedDocs: []string{"AGENTS.md"}, Evidence: []string{"read AGENTS.md"},
	}, holder); err != nil {
		t.Fatalf("project docs review: %v", err)
	}
	requireGitCallsOutsideSpan(t, log, "status")
}

func retargetReadyRecord(t *testing.T, stateRoot, repo string) issueops.IssueOpsRecord {
	t.Helper()
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "2819-child"})
	if err != nil {
		t.Fatal(err)
	}
	baseSHA := strings.TrimSpace(GitOut(repo, "rev-parse", "HEAD"))
	record.IssueURL = "https://github.com/example/issueops/issues/2819"
	record.BranchPrepare = &issueops.IssueOpsBranchPrepare{
		Provider: "github", IssueURL: record.IssueURL, Branch: "2819-child",
		BaseBranch: "main", BaseSHA: baseSHA, LinkVerified: true,
	}
	record.RemoteArtifact = &issueops.IssueOpsRemoteArtifactVerification{
		Provider: "github", Kind: "pr", URL: "https://github.com/example/issueops/pull/2820",
		Labels: []string{"enhancement"}, Assignees: []string{"maintainer"},
		VerifiedAt: "2026-09-23T00:00:00Z", TargetBranch: "main",
	}
	written, err := writeIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	return written
}
