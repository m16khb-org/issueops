package issueops

import (
	cycleapp "issueops/internal/application/issueopscycle"
	app "issueops/internal/application/issueopsowner"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/adapter/preflight"
	"issueops/internal/contract/issueops"
)

func TestRecordIssueOpsImplementationReviewValidation(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops")
	repo := gitInitedRepoForReviewTest(t)
	record, err := startIssueOpsFixture(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "83-review"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := RecordIssueOpsImplementationReview(stateRoot, record.ID, issueops.IssueOpsImplementationReviewRequest{Verdict: "approve"}); err == nil {
		t.Fatal("unknown verdict must be rejected")
	}
	if _, err := RecordIssueOpsImplementationReview(stateRoot, record.ID, issueops.IssueOpsImplementationReviewRequest{Verdict: "pass"}); err == nil {
		t.Fatal("pass without findings/evidence must be rejected")
	}
	valid := issueops.IssueOpsImplementationReviewRequest{
		Verdict: "pass", Findings: []string{"경계 조건 검토 완료"}, Evidence: []string{"go test ./... ok"},
		ReviewerHost: "codex", ReviewerModel: "gpt-6-sol", ReviewerEffort: "xhigh",
	}
	// C4b-F2: implement 이전 phase에서는 기록을 거부한다.
	if _, err := RecordIssueOpsImplementationReview(stateRoot, record.ID, valid); err == nil || !strings.Contains(err.Error(), "implement phase") {
		t.Fatalf("pre-implement recording must be rejected: %v", err)
	}
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) { rec.Phase = issueops.IssueOpsPhaseImplement })
	got, err := RecordIssueOpsImplementationReview(stateRoot, record.ID, valid)
	if err != nil {
		t.Fatal(err)
	}
	review := got.ImplementationReview
	if review == nil || review.Verdict != "pass" || review.ReviewerModel != "gpt-6-sol" {
		t.Fatalf("review must round-trip with audit fields: %+v", review)
	}
	// C4b-F1: 리뷰가 변경 집합 fingerprint를 봉인한다.
	if review.ReviewedFingerprint == "" {
		t.Fatalf("review must bind the reviewed change fingerprint: %+v", review)
	}
}

// gitInitedRepoForReviewTest는 변경 fingerprint가 계산 가능한 최소 git repo를
// 만든다(미추적 파일 1개 = 변경 집합).
func gitInitedRepoForReviewTest(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
		if code, _, stderr := preflight.GitCmd(repo, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "change.go"), []byte("package x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

// AC-05: execution이 있는 모든 모드에 fail-closed로 적용된다. execution이 없는
// 레코드(execution을 준비하기 전)만 면제다.
func TestImplementationReviewMissingAppliesToEveryExecutionMode(t *testing.T) {
	record := issueops.IssueOpsRecord{}
	if got := cycleapp.ImplementationReviewMissing(record, ""); got != "" {
		t.Fatalf("record without execution must not be gated: %q", got)
	}
	for _, mode := range []issueops.ExecutionMode{issueops.ExecutionModeDirect, issueops.ExecutionModeOrca} {
		record.Execution = &issueops.Execution{Mode: mode, Selection: selectionFixture(mode)}
		record.ImplementationReview = nil
		if got := cycleapp.ImplementationReviewMissing(record, ""); got != "implementation_review" {
			t.Fatalf("%s mode without review must be gated: %q", mode, got)
		}
	}
	record.Execution = &issueops.Execution{Mode: issueops.ExecutionModeOrca, Selection: selectionFixture(issueops.ExecutionModeOrca)}
	record.ImplementationReview = &issueops.IssueOpsImplementationReview{Verdict: "revise"}
	if got := cycleapp.ImplementationReviewMissing(record, ""); got != "implementation_review_verdict_revise" {
		t.Fatalf("non-pass verdict must be gated with its verdict: %q", got)
	}
	record.ImplementationReview.Verdict = "pass"
	if got := cycleapp.ImplementationReviewMissing(record, ""); got != "" {
		t.Fatalf("pass verdict must clear the gate: %q", got)
	}
	// C4b-F1: fingerprint가 현재 변경 집합과 다르면 stale로 거부.
	record.ImplementationReview.ReviewedFingerprint = "old"
	if got := cycleapp.ImplementationReviewMissing(record, "new"); got != "implementation_review_stale" {
		t.Fatalf("drifted fingerprint must be stale: %q", got)
	}
	record.ImplementationReview.ReviewedFingerprint = "new"
	if got := cycleapp.ImplementationReviewMissing(record, "new"); got != "" {
		t.Fatalf("matching fingerprint must clear the gate: %q", got)
	}
}

func TestStrictPRReadinessSurfacesImplementationReview(t *testing.T) {
	record := issueops.IssueOpsRecord{Execution: &issueops.Execution{Mode: issueops.ExecutionModeOrca, Selection: selectionFixture(issueops.ExecutionModeOrca)}}
	ready := IssueOpsPRReadiness(record)
	if !containsString(ready.Missing, "implementation_review") {
		t.Fatalf("strict readiness must surface the implementation review gate: %+v", ready.Missing)
	}
}

// 9단계 재편에서 direct가 기본 경로가 됐다. 검증 단계가 이 기록을 만들므로
// direct 사이클의 pr readiness도 리뷰 없이는 열리지 않아야 한다.
func TestDirectModeRequiresImplementationReviewForPR(t *testing.T) {
	record := issueops.IssueOpsRecord{Execution: &issueops.Execution{Mode: issueops.ExecutionModeDirect, Selection: selectionFixture(issueops.ExecutionModeDirect)}}
	if ready := IssueOpsPRReadiness(record); !containsString(ready.Missing, "implementation_review") {
		t.Fatalf("direct mode must surface the implementation review gate: %+v", ready.Missing)
	}
	record.ImplementationReview = &issueops.IssueOpsImplementationReview{Verdict: "pass"}
	if ready := IssueOpsPRReadiness(record); containsString(ready.Missing, "implementation_review") {
		t.Fatalf("a recorded pass review must clear the direct gate: %+v", ready.Missing)
	}
}

func TestOwnerCommandsIncludeImplementationReviewWithRuntimeInputs(t *testing.T) {
	repo := t.TempDir()
	worktree := filepath.Join(t.TempDir(), "83-review")
	record := issueops.IssueOpsRecord{
		ID: "io-000000000083", Repo: repo,
		BranchPrepare: &issueops.IssueOpsBranchPrepare{Provider: "github", BaseBranch: "main"},
		Execution: &issueops.Execution{
			Mode:      issueops.ExecutionModeOrca,
			Workspace: issueops.Workspace{SourceRoot: repo, Root: worktree, Branch: "83-review", BaseHead: "deadbeef"},
			Lease:     issueops.WriteLease{Generation: 1},
			Selection: selectionFixture(issueops.ExecutionModeOrca),
		},
	}
	for _, host := range []string{"codex", "claude", "omo"} {
		t.Run(host, func(t *testing.T) {
			commands := executionOwnerCommandsFor(record, issueops.ExecutionPrepareRequest{OwnerHost: host}, strings.Repeat("a", 64))
			if !strings.Contains(commands.ImplementationReview, "implementation-review record") ||
				!strings.Contains(commands.ImplementationReview, "--reviewer-model <REVIEWER_MODEL> --reviewer-effort <REVIEWER_EFFORT>") {
				t.Fatalf("owner command must accept actual runtime reviewer values: %s", commands.ImplementationReview)
			}
			if err := app.ValidateOwnerCatalog(commands); err != nil {
				t.Fatalf("implementation review command must match the catalog: %v", err)
			}
		})
	}
}

// fingerprint를 계산할 수 없는 사이클도 판정은 기록할 수 있고, 빈 봉인은
// fingerprint가 생기는 순간 stale로 잡힌다. project_docs_review와 같은 관용이며,
// 게이트를 모든 모드로 넓힌 뒤 탈출구 없는 교착을 만들지 않기 위한 것이다.
func TestImplementationReviewSealsAnEmptyFingerprintAndCatchesItLater(t *testing.T) {
	record := issueops.IssueOpsRecord{
		Execution:            &issueops.Execution{Mode: issueops.ExecutionModeDirect, Selection: selectionFixture(issueops.ExecutionModeDirect)},
		ImplementationReview: &issueops.IssueOpsImplementationReview{Verdict: "pass", ReviewedFingerprint: ""},
	}
	if got := cycleapp.ImplementationReviewMissing(record, ""); got != "" {
		t.Fatalf("an empty seal with no computable fingerprint must pass: %q", got)
	}
	if got := cycleapp.ImplementationReviewMissing(record, "now-computable"); got != "implementation_review_stale" {
		t.Fatalf("an empty seal must go stale once a fingerprint exists: %q", got)
	}
}
