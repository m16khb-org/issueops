package issueops

import (
	"strings"
	"testing"

	preflight "issueops/internal/adapter/preflight"
	"issueops/internal/contract/issueops"
)

// baseAdvancedRecord는 준비된 base가 있는 최소 strict readiness record다.
func baseAdvancedRecord(repo, baseBranch string) issueops.IssueOpsRecord {
	return issueops.IssueOpsRecord{
		OK: true, Repo: repo, Branch: "main", WorktreePath: repo,
		IssueURL:           "https://gitlab.example/group/project/-/issues/1",
		PlanPath:           "plans/demo.md",
		Intent:             issueOpsIntentContractForTest(),
		ArchitectureReview: issueOpsArchitectureReviewForTest(),
		BranchPrepare:      &issueops.IssueOpsBranchPrepare{Provider: "gitlab", IssueURL: "https://gitlab.example/group/project/-/issues/1", Branch: "main", BaseBranch: baseBranch, LinkVerified: true},
		AISlopCleanAt:      "2026-06-05T00:00:00Z",
		ProjectDocsReview: &issueops.IssueOpsProjectDocsReview{
			Verdict: "no-change", ReviewedDocs: []string{".issueops/CAUTIONS.md"},
		},
	}
}

func hasBaseAdvancedWarning(ready issueops.IssueOpsReadiness) bool {
	for _, warning := range ready.Warnings {
		if strings.HasPrefix(warning, "base_advanced:") {
			return true
		}
	}
	return false
}

// origin/<base>가 HEAD의 조상이 아니면 base가 앞서 나간 것이다. 이것은 경고이지
// 차단 키가 아니다 — PR 게이트 정책은 이 관측으로 바뀌지 않는다.
func TestStrictReadinessWarnsWhenPreparedBaseAdvanced(t *testing.T) {
	t.Parallel()

	repo := initIssueOpsRepo(t)
	record := baseAdvancedRecord(repo, "main")

	if ready := IssueOpsStrictPRReadiness(record); hasBaseAdvancedWarning(ready) {
		t.Fatalf("an up-to-date base must not warn: %+v", ready.Warnings)
	}

	// origin/main만 앞으로 옮긴다. 워크트리 HEAD는 그대로다.
	if code, _, stderr := preflight.GitCmd(repo, "commit", "-q", "--allow-empty", "-m", "upstream moves"); code != 0 {
		t.Fatalf("empty commit failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "push", "-q", "origin", "main"); code != 0 {
		t.Fatalf("push failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "reset", "--hard", "HEAD~1"); code != 0 {
		t.Fatalf("reset failed: %s", stderr)
	}

	ready := IssueOpsStrictPRReadiness(record)
	if !hasBaseAdvancedWarning(ready) {
		t.Fatalf("an advanced base must warn: %+v", ready.Warnings)
	}
	for _, key := range ready.Missing {
		if strings.Contains(key, "base_advanced") {
			t.Fatalf("base drift must not become a blocking key: %+v", ready.Missing)
		}
	}
}

// BaseBranch는 그대로 저장되므로 refs/heads/·origin/ 접두가 올 수 있다.
func TestStrictReadinessNormalizesBaseBranchPrefixes(t *testing.T) {
	t.Parallel()

	repo := initIssueOpsRepo(t)
	if code, _, stderr := preflight.GitCmd(repo, "commit", "-q", "--allow-empty", "-m", "upstream moves"); code != 0 {
		t.Fatalf("empty commit failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "push", "-q", "origin", "main"); code != 0 {
		t.Fatalf("push failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "reset", "--hard", "HEAD~1"); code != 0 {
		t.Fatalf("reset failed: %s", stderr)
	}
	for _, base := range []string{"refs/heads/main", "origin/main"} {
		if !hasBaseAdvancedWarning(IssueOpsStrictPRReadiness(baseAdvancedRecord(repo, base))) {
			t.Fatalf("base %q must normalize to origin/main and warn", base)
		}
	}
}

// 비교할 것이 없으면 추정하지 않는다. 둘 다 조용히 건너뛴다.
func TestStrictReadinessSkipsBaseObservationWithoutAComparableRef(t *testing.T) {
	t.Parallel()

	repo := initIssueOpsRepo(t)
	noPrepare := baseAdvancedRecord(repo, "main")
	noPrepare.BranchPrepare = nil
	if hasBaseAdvancedWarning(IssueOpsStrictPRReadiness(noPrepare)) {
		t.Fatal("a record without branch prepare has no base to compare")
	}
	// 로컬에 없는 base ref를 앞서 나간 base로 추정하지 않는다.
	if hasBaseAdvancedWarning(IssueOpsStrictPRReadiness(baseAdvancedRecord(repo, "nope"))) {
		t.Fatal("an absent ref must never be reported as advanced")
	}
}

// origin에서 base가 지워지면(삭제·이름 변경) prune fetch 뒤 base_missing을 경고한다.
// 그대로 두면 오래된 origin/<base>가 남아 PR 생성이 provider에서야 실패한다.
func TestStrictReadinessWarnsWhenBaseWasDeletedOnOrigin(t *testing.T) {
	t.Parallel()

	repo := initIssueOpsRepo(t)
	code, origin, stderr := preflight.GitCmd(repo, "remote", "get-url", "origin")
	if code != 0 {
		t.Fatalf("remote get-url failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "push", "-q", "origin", "main:release"); code != 0 {
		t.Fatalf("push failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "fetch", "-q", "origin"); code != 0 {
		t.Fatalf("fetch failed: %s", stderr)
	}
	record := baseAdvancedRecord(repo, "release")
	if hasWarningPrefix(IssueOpsStrictPRReadiness(record), "base_missing:") {
		t.Fatal("an existing base must not warn")
	}
	if code, _, stderr := preflight.GitCmd(strings.TrimSpace(origin), "branch", "-D", "release"); code != 0 {
		t.Fatalf("delete on origin failed: %s", stderr)
	}
	ready := IssueOpsStrictPRReadiness(record)
	if !hasWarningPrefix(ready, "base_missing: origin/release") || hasBaseAdvancedWarning(ready) {
		t.Fatalf("a base deleted on origin must warn base_missing only: %+v", ready.Warnings)
	}
}

func hasWarningPrefix(ready issueops.IssueOpsReadiness, prefix string) bool {
	for _, warning := range ready.Warnings {
		if strings.HasPrefix(warning, prefix) {
			return true
		}
	}
	return false
}
