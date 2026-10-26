package issueops

import (
	"strings"
	"testing"

	"issueops/internal/contract/issueops"
)

// Reviewer model/effort are recorded as audit fields, not as a finding, so
// the round's finding count stays the number of real findings and the
// tracked record still names the reviewer.
func TestTrackedPlanReviewNamesTheReviewerOutsideTheFindings(t *testing.T) {
	review := &issueops.IssueOpsDevilsAdvocateReview{
		Verdict: "pass", Findings: []string{"경계값을 확인했다"}, ReviewerModel: "claude-opus-5-5",
		History: []issueops.IssueOpsDevilsAdvocateRound{{Verdict: "revise", Findings: []string{"G2가 음수를 보지 않는다"}}},
	}
	got := RenderTrackedPlanReview(review)
	if !strings.Contains(got, "## 2차: 통과\n\n리뷰어: claude-opus-5-5, effort unknown\n") || strings.Count(got, "리뷰어:") != 1 {
		t.Fatalf("tracked review =\n%s", got)
	}
	if rounds := PlanReviewRounds(review, 0); rounds[1].Findings != 1 {
		t.Fatalf("rounds = %+v", rounds)
	}
}
