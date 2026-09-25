package issueopsreview

import (
	"strings"
	"testing"

	reviewcontract "issueops/internal/contract/issueopsreview"
)

func TestDesignReviewPreservesPreReadAndPostReadChecks(t *testing.T) {
	request := reviewcontract.DesignReviewRequest{
		ProblemSummary: " problem ", ProposedDesign: " design ",
		Verification: []string{" 설계 검토 완료: 대안과 위험 확인 "},
		RefactorPlan: " plan ", Alternatives: []string{" option "}, Risks: []string{" risk "}, Approved: true,
	}
	draft, err := PrepareDesignReview(request)
	if err != nil || draft.ProblemSummary != "problem" || draft.RefactorPlan != "plan" || draft.ReviewedAt != "" {
		t.Fatalf("prepared design review = %+v %v", draft, err)
	}
	review, err := FinalizeDesignReview(draft, "2026-09-24T00:00:00Z")
	if err != nil || review.ReviewedAt != "2026-09-24T00:00:00Z" {
		t.Fatalf("finalized design review = %+v %v", review, err)
	}
	request.OpenQuestions = []string{"still open"}
	if _, err := PrepareDesignReview(request); err == nil || !strings.Contains(err.Error(), "must not have open_questions") {
		t.Fatalf("approved review with question accepted: %v", err)
	}
	request.OpenQuestions = nil
	request.RefactorPlan = ""
	draft, err = PrepareDesignReview(request)
	if err != nil {
		t.Fatalf("base validation must leave approval gate until after readiness: %v", err)
	}
	if _, err := FinalizeDesignReview(draft, "now"); err == nil || !strings.Contains(err.Error(), "requires refactor_plan") {
		t.Fatalf("incomplete approved review accepted: %v", err)
	}
}

func TestHasDesignReviewEvidence(t *testing.T) {
	for _, test := range []struct {
		values []string
		want   bool
	}{
		{[]string{"design review checked alternatives and risks"}, true},
		{[]string{"design audit complete"}, true},
		{[]string{"design evaluated"}, true},
		{[]string{"설계 검토 완료"}, true},
		{[]string{"설계 검수 완료"}, true},
		{[]string{"no evidence here"}, false},
		{nil, false},
		{[]string{""}, false},
		{[]string{"code review done", "design review done"}, true},
	} {
		if got := HasDesignReviewEvidence(test.values); got != test.want {
			t.Errorf("HasDesignReviewEvidence(%v) = %v, want %v", test.values, got, test.want)
		}
	}
}
