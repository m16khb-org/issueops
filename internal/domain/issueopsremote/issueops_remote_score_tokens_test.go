package remote

import (
	"reflect"
	"testing"
)

func TestIssueOpsRemoteOverlapUsesSmallerTokenSet(t *testing.T) {
	left := map[string]bool{"refactor": true, "coverage": true, "issueops": true}
	right := map[string]bool{"coverage": true, "issueops": true}

	if got := issueOpsRemoteOverlap(left, right); got != 1 {
		t.Fatalf("issueOpsRemoteOverlap() = %v, want 1", got)
	}
}

func TestIssueOpsRemoteOverlapRejectsEmptyInputs(t *testing.T) {
	if got := issueOpsRemoteOverlap(nil, map[string]bool{"coverage": true}); got != 0 {
		t.Fatalf("issueOpsRemoteOverlap(left empty) = %v, want 0", got)
	}
	if got := issueOpsRemoteOverlap(map[string]bool{"coverage": true}, nil); got != 0 {
		t.Fatalf("issueOpsRemoteOverlap(right empty) = %v, want 0", got)
	}
}

func TestIssueOpsRemoteKeywordMatchesKoreanPrefixAndEnglishExact(t *testing.T) {
	tests := []struct {
		token, keyword string
		want           bool
	}{
		{"오류가", "오류", true},
		{"재현되지", "재현", true},
		{"오류", "오류", true},
		{"add", "add", true},
		{"address", "add", false},
		{"adds", "add", false},
		{"여기오류", "오류", false},
		{"오", "오류", false},
	}
	for _, tt := range tests {
		if got := issueOpsRemoteKeywordMatches(tt.token, tt.keyword); got != tt.want {
			t.Errorf("issueOpsRemoteKeywordMatches(%q, %q) = %v, want %v", tt.token, tt.keyword, got, tt.want)
		}
	}
}

func TestIssueOpsRemoteLabelEvidenceCountsEachTokenOncePerRule(t *testing.T) {
	tokens := issueOpsRemoteTokens("오류가 오류를 failure add address 문서 readme")
	evidence := issueOpsRemoteLabelEvidence(tokens)

	want := map[string][]string{
		"bug":           {"failure", "오류가", "오류를"},
		"enhancement":   {"add"},
		"documentation": {"readme", "문서"},
	}
	for label, wantTokens := range want {
		if !reflect.DeepEqual(evidence.matched[label], wantTokens) {
			t.Errorf("matched[%s] = %v, want %v", label, evidence.matched[label], wantTokens)
		}
	}
	if evidence.total != 6 {
		t.Errorf("total = %d, want 6", evidence.total)
	}
}

func TestIssueOpsRemoteLabelEvidenceScoreAbstainsBelowMinimumEvidence(t *testing.T) {
	bug := IssueOpsRemoteLabelCandidate{Name: " BUG "}
	one := issueOpsRemoteLabelEvidence(issueOpsRemoteTokens("회귀"))
	score, matched, ok := issueOpsRemoteLabelEvidenceScore(one, bug)
	if !ok || score != 0 || !reflect.DeepEqual(matched, []string{"회귀"}) {
		t.Fatalf("one evidence: score=%v matched=%v ok=%v, want 0 [회귀] true", score, matched, ok)
	}

	two := issueOpsRemoteLabelEvidence(issueOpsRemoteTokens("회귀 failure"))
	if score, _, _ := issueOpsRemoteLabelEvidenceScore(two, bug); score != 1 {
		t.Fatalf("two evidence: score=%v, want 1", score)
	}

	if _, _, ok := issueOpsRemoteLabelEvidenceScore(two, IssueOpsRemoteLabelCandidate{Name: "question"}); ok {
		t.Fatal("label without a keyword rule must not use the evidence score")
	}
}
