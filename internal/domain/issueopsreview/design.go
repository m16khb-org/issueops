package issueopsreview

import (
	"errors"
	"fmt"
	"strings"

	reviewcontract "issueops/internal/contract/issueopsreview"
	"issueops/internal/domain/policy"
)

var ErrMissingDesignReviewEvidence = errors.New("approved design review requires design review evidence")

func PrepareDesignReview(req reviewcontract.DesignReviewRequest) (reviewcontract.DesignReview, error) {
	problemSummary := strings.TrimSpace(req.ProblemSummary)
	if problemSummary == "" {
		return reviewcontract.DesignReview{}, fmt.Errorf("problem_summary is required")
	}
	proposedDesign := strings.TrimSpace(req.ProposedDesign)
	if proposedDesign == "" {
		return reviewcontract.DesignReview{}, fmt.Errorf("proposed_design is required")
	}
	verification := cleanDesignValues(req.Verification)
	if len(verification) == 0 {
		return reviewcontract.DesignReview{}, fmt.Errorf("verification is required")
	}
	openQuestions := cleanDesignValues(req.OpenQuestions)
	if req.Approved && len(openQuestions) > 0 {
		return reviewcontract.DesignReview{}, fmt.Errorf("approved design review must not have open_questions")
	}
	return reviewcontract.DesignReview{
		ProblemSummary: policy.RedactFreeform(problemSummary),
		ProposedDesign: policy.RedactFreeform(proposedDesign),
		RefactorPlan:   policy.RedactFreeform(strings.TrimSpace(req.RefactorPlan)),
		Alternatives:   cleanDesignValues(req.Alternatives), Risks: cleanDesignValues(req.Risks),
		Verification: verification, OpenQuestions: openQuestions, Approved: req.Approved,
	}, nil
}

func FinalizeDesignReview(review reviewcontract.DesignReview, reviewedAt string) (reviewcontract.DesignReview, error) {
	if review.Approved && review.RefactorPlan == "" {
		return reviewcontract.DesignReview{}, fmt.Errorf("approved design review requires refactor_plan")
	}
	if review.Approved && len(review.Alternatives) == 0 {
		return reviewcontract.DesignReview{}, fmt.Errorf("approved design review requires alternatives")
	}
	if review.Approved && len(review.Risks) == 0 {
		return reviewcontract.DesignReview{}, fmt.Errorf("approved design review requires risks")
	}
	if review.Approved && !HasDesignReviewEvidence(review.Verification) {
		return reviewcontract.DesignReview{}, ErrMissingDesignReviewEvidence
	}
	review.ReviewedAt = reviewedAt
	return review, nil
}

func HasDesignReviewEvidence(values []string) bool {
	for _, value := range values {
		text := strings.ToLower(strings.TrimSpace(value))
		if text == "" {
			continue
		}
		if strings.Contains(text, "design") && (strings.Contains(text, "review") || strings.Contains(text, "audit") || strings.Contains(text, "evaluat")) {
			return true
		}
		if strings.Contains(text, "설계") && (strings.Contains(text, "검수") || strings.Contains(text, "검토")) {
			return true
		}
	}
	return false
}

func NonPlanPrepMissing(missing []string) []string {
	out := []string{}
	for _, item := range missing {
		if !strings.HasPrefix(item, "plan_prep_") {
			out = append(out, item)
		}
	}
	return out
}

func cleanDesignValues(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || strings.Contains(value, "\x00") || seen[value] {
			continue
		}
		value = policy.RedactFreeform(value)
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
