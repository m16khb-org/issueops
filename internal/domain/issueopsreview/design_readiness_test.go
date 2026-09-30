package issueopsreview

import (
	"reflect"
	"testing"

	reviewcontract "issueops/internal/contract/issueopsreview"
)

func TestDesignReviewMissingPreservesOrderedApprovalGates(t *testing.T) {
	if got := DesignReviewMissing(nil); !reflect.DeepEqual(got, []string{"design_review"}) {
		t.Fatalf("missing review=%v", got)
	}
	review := &reviewcontract.DesignReview{
		ProblemSummary: "problem", ProposedDesign: "design", Verification: []string{"go test"},
		Approved: true, OpenQuestions: []string{"unresolved"},
	}
	want := []string{"design_review_evidence", "design_open_questions", "refactor_plan", "alternatives", "risks"}
	if got := DesignReviewMissing(review); !reflect.DeepEqual(got, want) {
		t.Fatalf("missing=%v, want %v", got, want)
	}
	review.Verification = []string{"design review checked risks"}
	review.OpenQuestions = nil
	review.RefactorPlan = "plan"
	review.Alternatives = []string{"other"}
	review.Risks = []string{"risk"}
	if got := DesignReviewMissing(review); len(got) != 0 {
		t.Fatalf("approved review missing=%v", got)
	}
}
