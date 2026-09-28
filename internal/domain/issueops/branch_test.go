package issueops

import (
	"strings"
	"testing"
)

func TestValidateBranchErrorUsesNeutralExamples(t *testing.T) {
	err := ValidateBranch("dogfood-lifecycle")
	if err == nil {
		t.Fatal("expected branch without issue-number prefix to be rejected")
	}
	if !strings.Contains(err.Error(), "issue number followed by a hyphen") {
		t.Fatalf("expected rule text, got %v", err)
	}
	// 안내 문구의 예시는 이 저장소가 아닌 외부 프로젝트 이슈를 참조해선 안 된다.
	// dogfooding 중 실제로 혼란을 일으킨 회귀 방지(#dogfood-whoami-record-flags).
	for _, foreign := range []string{"fanza", "grpc-ai"} {
		if strings.Contains(err.Error(), foreign) {
			t.Fatalf("branch error must not reference unrelated projects: %v", err)
		}
	}
}

func TestValidateBranchPreservesAcceptedAndRejectedNames(t *testing.T) {
	for _, branch := range []string{"", "  ", "123-fix", " 123-fix ", "0-feature/path"} {
		if err := ValidateBranch(branch); err != nil {
			t.Fatalf("accepted branch %q: %v", branch, err)
		}
	}
	for _, branch := range []string{"main", "abc-fix", "123-", "/123-fix", "123-fix..bad", "123-two words", "123-tab\tword", "１２３-fix"} {
		if err := ValidateBranch(branch); err == nil {
			t.Fatalf("invalid branch %q accepted", branch)
		}
	}
}
