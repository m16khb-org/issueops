package issueopsreview

import (
	"strings"

	reviewcontract "issueops/internal/contract/issueopsreview"
)

func DesignReviewMissing(review *reviewcontract.DesignReview) []string {
	if review == nil {
		return []string{"design_review"}
	}
	missing := []string{}
	if strings.TrimSpace(review.ProblemSummary) == "" {
		missing = append(missing, "problem_summary")
	}
	if strings.TrimSpace(review.ProposedDesign) == "" {
		missing = append(missing, "proposed_design")
	}
	if len(cleanDesignValues(review.Verification)) == 0 {
		missing = append(missing, "design_verification")
	}
	if review.Approved && !HasDesignReviewEvidence(review.Verification) {
		missing = append(missing, "design_review_evidence")
	}
	if !review.Approved {
		missing = append(missing, "design_approval")
	}
	if len(cleanDesignValues(review.OpenQuestions)) > 0 {
		missing = append(missing, "design_open_questions")
	}
	if review.Approved && strings.TrimSpace(review.RefactorPlan) == "" {
		missing = append(missing, "refactor_plan")
	}
	if review.Approved && len(cleanDesignValues(review.Alternatives)) == 0 {
		missing = append(missing, "alternatives")
	}
	if review.Approved && len(cleanDesignValues(review.Risks)) == 0 {
		missing = append(missing, "risks")
	}
	return missing
}
