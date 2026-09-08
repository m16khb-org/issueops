package issueops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/contract/issueops"
)

// gitRepoWithProjectDocsForTest는 committed 문서 하나(변경 집합 밖)와
// untracked 변경 둘(구현 파일, 운영 문서)을 가진 최소 repo를 만든다.
func gitRepoWithProjectDocsForTest(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
		if code, _, stderr := preflightGitForReviewTest(repo, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	writeRepoFileForTest(t, repo, ".issueops/ADR.md", "# adr\n")
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "base"}} {
		if code, _, stderr := preflightGitForReviewTest(repo, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	writeRepoFileForTest(t, repo, "change.go", "package x\n")
	writeRepoFileForTest(t, repo, ".issueops/CAUTIONS.md", "# cautions\n")
	return repo
}

func writeRepoFileForTest(t *testing.T, repo, rel, body string) {
	t.Helper()
	abs := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRecordIssueOpsProjectDocsReviewValidation(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops")
	repo := gitRepoWithProjectDocsForTest(t)
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "90-docs"})
	if err != nil {
		t.Fatal(err)
	}
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) { rec.Phase = IssueOpsPhaseImplement })

	if _, err := RecordIssueOpsProjectDocsReview(stateRoot, record.ID, IssueOpsProjectDocsReviewRequest{Verdict: "done"}); err == nil {
		t.Fatal("unknown verdict must be rejected")
	}
	if _, err := RecordIssueOpsProjectDocsReview(stateRoot, record.ID, IssueOpsProjectDocsReviewRequest{Verdict: "no-change"}); err == nil {
		t.Fatal("verdict without evidence must be rejected")
	}
	if _, err := RecordIssueOpsProjectDocsReview(stateRoot, record.ID, IssueOpsProjectDocsReviewRequest{
		Verdict: "updated", Evidence: []string{"CAUTIONS 확인"},
	}); err == nil {
		t.Fatal("updated verdict without a doc path must be rejected")
	}
	if _, err := RecordIssueOpsProjectDocsReview(stateRoot, record.ID, IssueOpsProjectDocsReviewRequest{
		Verdict: "no-change", Docs: []string{".issueops/CAUTIONS.md"}, Evidence: []string{"확인함"},
	}); err == nil {
		t.Fatal("no-change verdict must not carry updated docs")
	}
	// 연극 방지: 변경 집합에 없는 문서를 갱신했다고 주장하면 거부한다.
	if _, err := RecordIssueOpsProjectDocsReview(stateRoot, record.ID, IssueOpsProjectDocsReviewRequest{
		Verdict: "updated", Docs: []string{".issueops/ADR.md"}, Evidence: []string{"ADR 갱신"},
	}); err == nil || !strings.Contains(err.Error(), "change set") {
		t.Fatalf("doc outside the change set must be rejected: %v", err)
	}
	got, err := RecordIssueOpsProjectDocsReview(stateRoot, record.ID, IssueOpsProjectDocsReviewRequest{
		Verdict: "updated", Docs: []string{".issueops/CAUTIONS.md"}, Evidence: []string{"재발 함정 기록"},
	})
	if err != nil {
		t.Fatal(err)
	}
	review := got.ProjectDocsReview
	if review == nil || review.Verdict != "updated" || len(review.Docs) != 1 {
		t.Fatalf("review must round-trip: %+v", review)
	}
	if review.ReviewedFingerprint == "" {
		t.Fatalf("review must bind the reviewed change fingerprint: %+v", review)
	}
}

func TestRecordIssueOpsProjectDocsReviewRejectsPreImplementPhase(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops")
	repo := gitRepoWithProjectDocsForTest(t)
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "91-docs"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RecordIssueOpsProjectDocsReview(stateRoot, record.ID, IssueOpsProjectDocsReviewRequest{
		Verdict: "no-change", ReviewedDocs: []string{".issueops/ADR.md"}, Evidence: []string{"문서 영향 없음"},
	})
	if err == nil || !strings.Contains(err.Error(), "implement phase") {
		t.Fatalf("pre-implement recording must be rejected: %v", err)
	}
}

// project-docs 게이트는 implementation review와 달리 direct/orca 양쪽에 걸린다.
func TestProjectDocsReviewMissingAppliesToBothModes(t *testing.T) {
	record := issueops.IssueOpsRecord{Execution: &issueops.Execution{Mode: issueops.ExecutionModeDirect}}
	if got := projectDocsReviewMissing(record, ""); got != "project_docs_review" {
		t.Fatalf("direct mode must also be gated: %q", got)
	}
	record.Execution.Mode = issueops.ExecutionModeOrca
	if got := projectDocsReviewMissing(record, ""); got != "project_docs_review" {
		t.Fatalf("orca mode must be gated: %q", got)
	}
	record.ProjectDocsReview = &issueops.IssueOpsProjectDocsReview{Verdict: "no-change"}
	if got := projectDocsReviewMissing(record, ""); got != "" {
		t.Fatalf("recorded review must clear the gate: %q", got)
	}
	record.ProjectDocsReview.ReviewedFingerprint = "old"
	if got := projectDocsReviewMissing(record, "new"); got != "project_docs_review_stale" {
		t.Fatalf("drifted fingerprint must be stale: %q", got)
	}
	record.ProjectDocsReview.ReviewedFingerprint = "new"
	if got := projectDocsReviewMissing(record, "new"); got != "" {
		t.Fatalf("matching fingerprint must clear the gate: %q", got)
	}
}

func TestPRReadinessSurfacesProjectDocsReview(t *testing.T) {
	record := issueops.IssueOpsRecord{Execution: &issueops.Execution{Mode: issueops.ExecutionModeDirect}}
	if ready := IssueOpsPRReadiness(record); !containsString(ready.Missing, "project_docs_review") {
		t.Fatalf("PR readiness must surface the project docs gate: %+v", ready.Missing)
	}
}

// no-change 판정은 "무엇을 읽었는가"를 경로로 남겨야 한다. 자유 텍스트 evidence만으로는
// "대조했으나 없음"과 "대조하지 않음"을 코드가 구분할 수 없기 때문이다.
func TestRecordIssueOpsProjectDocsReviewNoChangeRequiresReviewedDocs(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops")
	repo := gitRepoWithProjectDocsForTest(t)
	writeRepoFileForTest(t, repo, "AGENTS.md", "# agents\n")
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "92-docs"})
	if err != nil {
		t.Fatal(err)
	}
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) { rec.Phase = IssueOpsPhaseImplement })

	if _, err := RecordIssueOpsProjectDocsReview(stateRoot, record.ID, IssueOpsProjectDocsReviewRequest{
		Verdict: "no-change", Evidence: []string{"대조했으나 없음"},
	}); err == nil || !strings.Contains(err.Error(), "--reviewed-doc") {
		t.Fatalf("no-change without reviewed docs must be rejected: %v", err)
	}
	for _, tc := range []struct{ doc, want string }{
		{doc: "change.go", want: "project doc"},
		{doc: ".issueops/MISSING.md", want: "does not exist"},
		{doc: "../outside.md", want: "inside the worktree"},
	} {
		if _, err := RecordIssueOpsProjectDocsReview(stateRoot, record.ID, IssueOpsProjectDocsReviewRequest{
			Verdict: "no-change", ReviewedDocs: []string{tc.doc}, Evidence: []string{"대조"},
		}); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("reviewed doc %q must be rejected with %q: %v", tc.doc, tc.want, err)
		}
	}
	got, err := RecordIssueOpsProjectDocsReview(stateRoot, record.ID, IssueOpsProjectDocsReviewRequest{
		Verdict:      "no-change",
		ReviewedDocs: []string{filepath.Join(repo, ".issueops", "ADR.md"), "AGENTS.md"},
		Evidence:     []string{"ADR과 AGENTS를 대조했으나 남길 결정 없음"},
	})
	if err != nil {
		t.Fatal(err)
	}
	review := got.ProjectDocsReview
	if review == nil || len(review.ReviewedDocs) != 2 || review.ReviewedDocs[0] != ".issueops/ADR.md" || review.ReviewedDocs[1] != "AGENTS.md" {
		t.Fatalf("reviewed docs must round-trip as repo-relative paths: %+v", review)
	}
}

// updated 판정도 읽은 문서를 함께 남길 수 있고, 같은 경로 규칙을 따른다.
func TestRecordIssueOpsProjectDocsReviewUpdatedAcceptsReviewedDocs(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops")
	repo := gitRepoWithProjectDocsForTest(t)
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "93-docs"})
	if err != nil {
		t.Fatal(err)
	}
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) { rec.Phase = IssueOpsPhaseImplement })
	got, err := RecordIssueOpsProjectDocsReview(stateRoot, record.ID, IssueOpsProjectDocsReviewRequest{
		Verdict: "updated", Docs: []string{".issueops/CAUTIONS.md"}, ReviewedDocs: []string{".issueops/ADR.md"},
		Evidence: []string{"재발 함정 기록"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectDocsReview == nil || len(got.ProjectDocsReview.ReviewedDocs) != 1 {
		t.Fatalf("updated verdict must keep reviewed docs: %+v", got.ProjectDocsReview)
	}
}

// 문서 반영 게이트는 execution lease 유무와 무관하다. lease 없이 implement 이후
// phase에 도달한 record도 publication 전에 판정을 남겨야 한다.
func TestProjectDocsReviewMissingGatesRecordsWithoutExecution(t *testing.T) {
	record := issueops.IssueOpsRecord{}
	if got := projectDocsReviewMissing(record, ""); got != "project_docs_review" {
		t.Fatalf("record without execution must still be gated: %q", got)
	}
	record.ProjectDocsReview = &issueops.IssueOpsProjectDocsReview{Verdict: "no-change", ReviewedFingerprint: "old"}
	if got := projectDocsReviewMissing(record, "new"); got != "project_docs_review_stale" {
		t.Fatalf("stale review without execution must be reported: %q", got)
	}
}
