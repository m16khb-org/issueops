package issueopsreview

import (
	"strings"
	"testing"

	reviewcontract "issueops/internal/contract/issueopsreview"
)

func TestArchitectureReviewPreservesPreReadAndPostReadChecks(t *testing.T) {
	request := reviewcontract.ArchitectureReviewRequest{
		ProblemSummary: " problem ", ProposedArchitecture: " design ",
		Verification: []string{" 아키텍처 검토 완료: 대안과 위험 확인 "},
		RefactorPlan: " plan ", Alternatives: []string{" option "}, Risks: []string{" risk "}, Approved: true, ReviewerContext: " Subagent ",
	}
	draft, err := PrepareArchitectureReview(request)
	if err != nil || draft.ProblemSummary != "problem" || draft.RefactorPlan != "plan" || draft.ReviewedAt != "" {
		t.Fatalf("prepared architecture review = %+v %v", draft, err)
	}
	review, err := FinalizeArchitectureReview(draft, "2026-09-24T00:00:00Z")
	if err != nil || review.ReviewedAt != "2026-09-24T00:00:00Z" {
		t.Fatalf("finalized architecture review = %+v %v", review, err)
	}
	request.OpenQuestions = []string{"still open"}
	if _, err := PrepareArchitectureReview(request); err == nil || !strings.Contains(err.Error(), "must not have open_questions") {
		t.Fatalf("approved review with question accepted: %v", err)
	}
	request.OpenQuestions = nil
	request.RefactorPlan = ""
	draft, err = PrepareArchitectureReview(request)
	if err != nil {
		t.Fatalf("base validation must leave approval gate until after readiness: %v", err)
	}
	if _, err := FinalizeArchitectureReview(draft, "now"); err == nil || !strings.Contains(err.Error(), "requires refactor_plan") {
		t.Fatalf("incomplete approved review accepted: %v", err)
	}
}

func TestHasArchitectureReviewEvidence(t *testing.T) {
	for _, test := range []struct {
		values []string
		want   bool
	}{
		{[]string{"architecture review checked alternatives and risks"}, true},
		{[]string{"architecture audit complete"}, true},
		{[]string{"architecture evaluated"}, true},
		{[]string{"아키텍처 검토 완료"}, true},
		{[]string{"아키텍처 검수 완료"}, true},
		{[]string{"no evidence here"}, false},
		{nil, false},
		{[]string{""}, false},
		{[]string{"code review done", "architecture review done"}, true},
	} {
		if got := HasArchitectureReviewEvidence(test.values); got != test.want {
			t.Errorf("HasArchitectureReviewEvidence(%v) = %v, want %v", test.values, got, test.want)
		}
	}
}
