package issueopsreview

import (
	"strings"

	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsdomain "issueops/internal/domain/issueops"
)

// ArchitectureReviewMissing 함수는 아키텍처 리뷰가 게이트를 통과하기 위해 아직 채워야 할
// 항목 키를 돌려준다. 리뷰가 없으면 architecture_review 하나만 담긴다.
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
	missing = append(missing, architectureApprovalMissing(review)...)
	if len(cleanArchitectureValues(review.OpenQuestions)) > 0 {
		missing = append(missing, "architecture_open_questions")
	}
	if review.Approved {
		missing = append(missing, architectureApprovedPlanMissing(review)...)
	}
	return missing
}

// architectureApprovalMissing은 승인 여부에 따라 빠진 승인 근거 키를 돌려준다.
func architectureApprovalMissing(review *reviewcontract.ArchitectureReview) []string {
	if !review.Approved {
		return []string{"architecture_approval"}
	}
	if !hasArchitectureReviewEvidence(review.Verification) {
		return []string{"architecture_review_evidence"}
	}
	return nil
}

// architectureApprovedPlanMissing은 승인된 리뷰에서 비어 있는 계획 필드를 돌려준다.
func architectureApprovedPlanMissing(review *reviewcontract.ArchitectureReview) []string {
	var missing []string
	if strings.TrimSpace(review.RefactorPlan) == "" {
		missing = append(missing, "refactor_plan")
	}
	if len(cleanArchitectureValues(review.Alternatives)) == 0 {
		missing = append(missing, "alternatives")
	}
	if len(cleanArchitectureValues(review.Risks)) == 0 {
		missing = append(missing, "risks")
	}
	return missing
}

// ArchitectureReviewStale 함수는 승인한 아키텍처 문서가 지금 문서와 달라졌는지 말한다.
// 승인 기록이 없거나 아직 승인되지 않았으면 다른 키가 이미 막고 있으므로 false다.
// 부모 리뷰를 승계한 위임 자식은 자기 문서를 검토하지 않았으므로 digest 게이트에서 빠진다.
func ArchitectureReviewStale(review *reviewcontract.ArchitectureReview, currentDigest string, digestErr error) bool {
	if review == nil || !review.Approved || review.ReviewerContext == issueopsdomain.ParentReviewPattern {
		return false
	}
	return strings.TrimSpace(review.ReviewedArchitectureDigest) == "" || digestErr != nil ||
		!strings.EqualFold(currentDigest, review.ReviewedArchitectureDigest)
}
