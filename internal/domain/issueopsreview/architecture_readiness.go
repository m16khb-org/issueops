package issueopsreview

import (
	"strings"

	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsdomain "issueops/internal/domain/issueops"
)

func ArchitectureReviewMissing(review *reviewcontract.ArchitectureReview) []string {
	if review == nil {
		return []string{"architecture_review"}
	}
	missing := []string{}
	if strings.TrimSpace(review.ProblemSummary) == "" {
		missing = append(missing, "problem_summary")
	}
	if strings.TrimSpace(review.ProposedArchitecture) == "" {
		missing = append(missing, "proposed_architecture")
	}
	if len(cleanArchitectureValues(review.Verification)) == 0 {
		missing = append(missing, "architecture_verification")
	}
	if review.Approved && !HasArchitectureReviewEvidence(review.Verification) {
		missing = append(missing, "architecture_review_evidence")
	}
	if !review.Approved {
		missing = append(missing, "architecture_approval")
	}
	if len(cleanArchitectureValues(review.OpenQuestions)) > 0 {
		missing = append(missing, "architecture_open_questions")
	}
	if review.Approved && strings.TrimSpace(review.RefactorPlan) == "" {
		missing = append(missing, "refactor_plan")
	}
	if review.Approved && len(cleanArchitectureValues(review.Alternatives)) == 0 {
		missing = append(missing, "alternatives")
	}
	if review.Approved && len(cleanArchitectureValues(review.Risks)) == 0 {
		missing = append(missing, "risks")
	}
	return missing
}

// ArchitectureReviewStale은 승인한 아키텍처 문서가 지금 문서와 달라졌는지 말한다.
// 승인 기록이 없거나 아직 승인되지 않았으면 다른 키가 이미 막고 있으므로 false다.
// 부모 리뷰를 승계한 위임 자식은 자기 문서를 검토하지 않았으므로 digest 게이트에서 빠진다.
func ArchitectureReviewStale(review *reviewcontract.ArchitectureReview, currentDigest string, digestErr error) bool {
	if review == nil || !review.Approved || review.ReviewerContext == issueopsdomain.ParentReviewPattern {
		return false
	}
	return strings.TrimSpace(review.ReviewedArchitectureDigest) == "" || digestErr != nil ||
		!strings.EqualFold(currentDigest, review.ReviewedArchitectureDigest)
}
