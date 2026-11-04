package issueopscycle

import (
	"strings"

	model "issueops/internal/contract/issueops"
	reviewcontract "issueops/internal/contract/issueopsreview"
	cycledomain "issueops/internal/domain/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
	cycleport "issueops/internal/port/issueopscycle"
)

// ImplementationReviewMissing 함수는 구현 리뷰 증거가 없거나 현재 지문과 맞지 않으면 누락 키를, 충족되면 빈 문자열을 돌려준다.
func ImplementationReviewMissing(record model.Record, currentFingerprint string) string {
	evidence := reviewcontract.ReviewGateEvidence{}
	if review := record.ImplementationReview; review != nil {
		evidence = reviewcontract.ReviewGateEvidence{
			Present: true, Verdict: review.Verdict, ReviewedFingerprint: review.ReviewedFingerprint,
		}
	}
	return reviewdomain.ImplementationReviewMissing(record.Execution != nil, evidence, currentFingerprint)
}

// ProjectDocsReviewMissing 함수는 프로젝트 문서 리뷰 증거가 없거나 현재 지문과 맞지 않으면 누락 키를, 충족되면 빈 문자열을 돌려준다.
func ProjectDocsReviewMissing(record model.Record, currentFingerprint string) string {
	evidence := reviewcontract.ReviewGateEvidence{}
	if review := record.ProjectDocsReview; review != nil {
		evidence = reviewcontract.ReviewGateEvidence{Present: true, ReviewedFingerprint: review.ReviewedFingerprint}
	}
	return reviewdomain.ProjectDocsReviewMissing(evidence, currentFingerprint)
}

func localPRReadinessMissing(record model.Record, observations cycleport.ReadinessObservations) []string {
	missing := baseImplementationMissing(record)
	if strings.TrimSpace(record.WorktreePath) == "" {
		missing = append(missing, "worktree_path")
	}
	if strings.TrimSpace(record.PlanPath) != "" && !observations.PlanPathExists(cycledomain.PlanExistenceRoot(record), record.PlanPath) {
		missing = append(missing, "plan_exists")
	}
	if !observations.PlanInLinkedWorktree(record) {
		missing = append(missing, "plan_in_worktree")
	}
	if strings.TrimSpace(record.AISlopCleanAt) == "" {
		missing = append(missing, "ai_slop_clean")
	}
	if reviewMissing := ImplementationReviewMissing(record, ""); reviewMissing != "" {
		missing = append(missing, reviewMissing)
	}
	if docsMissing := ProjectDocsReviewMissing(record, ""); docsMissing != "" {
		missing = append(missing, docsMissing)
	}
	if hasUnresolvedContractFeedback(record) {
		missing = append(missing, "contract_feedback_issue_update")
	}
	return missing
}
