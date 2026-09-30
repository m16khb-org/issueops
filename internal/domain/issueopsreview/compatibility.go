package issueopsreview

import (
	"fmt"
	"strings"

	reviewcontract "issueops/internal/contract/issueopsreview"
	"issueops/internal/domain/policy"
)

func ValidateCompatibilityReview(req reviewcontract.CompatibilityReviewRequest, reviewedAt string) (reviewcontract.CompatibilityReview, error) {
	backwardCompatibility, err := requiredReviewValues("backward_compatibility", req.BackwardCompatibility)
	if err != nil {
		return reviewcontract.CompatibilityReview{}, err
	}
	sideEffects, err := requiredReviewValues("side_effects", req.SideEffects)
	if err != nil {
		return reviewcontract.CompatibilityReview{}, err
	}
	verification, err := requiredReviewValues("verification", req.Verification)
	if err != nil {
		return reviewcontract.CompatibilityReview{}, err
	}
	rollbackPlan := strings.TrimSpace(req.RollbackPlan)
	if rollbackPlan == "" {
		return reviewcontract.CompatibilityReview{}, fmt.Errorf("rollback_plan is required")
	}
	blockers := cleanReviewValues(req.Blockers)
	if req.Approved && len(blockers) > 0 {
		return reviewcontract.CompatibilityReview{}, fmt.Errorf("approved compatibility review must not have blockers")
	}
	return reviewcontract.CompatibilityReview{
		BackwardCompatibility: backwardCompatibility, SideEffects: sideEffects,
		RollbackPlan: policy.RedactFreeform(rollbackPlan), Verification: verification,
		Blockers: blockers, Approved: req.Approved, ReviewedAt: reviewedAt,
	}, nil
}

func requiredReviewValues(field string, values []string) ([]string, error) {
	out := cleanReviewValues(values)
	if len(out) == 0 {
		return nil, fmt.Errorf("%s requires at least one entry", field)
	}
	return out, nil
}
