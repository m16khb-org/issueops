package issueops

import (
	"strings"
	"testing"
)

// AC-04: owner prompt 시작 절차 3은 봉인된 intent.md를 요청자 의도 계약으로 먼저 읽고,
// issue body와 충돌하면 mutation 없이 blocker를 보고하라고 지시한다.
func TestExecutionOwnerPromptReadsSealedIntentBeforeImplementing(t *testing.T) {
	for _, want := range []string{
		"manifest에 intent가 있으면",
		"intent.md",
		"요청자 의도 계약",
		"구현 계약",
		"blocker",
		"intent-vs-issue mismatch",
	} {
		if !strings.Contains(executionOwnerPromptTemplate, want) {
			t.Fatalf("owner prompt template must contain %q", want)
		}
	}
	if strings.Contains(executionOwnerPromptTemplate, "{WORKTREE_ROOT}/.issueops/artifact/") {
		t.Fatal("owner prompt template must not point at the legacy artifact directory")
	}
}
