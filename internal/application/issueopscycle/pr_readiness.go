package issueopscycle

import (
	"strings"

	model "issueops/internal/contract/issueops"
	reviewcontract "issueops/internal/contract/issueopsreview"
	cycledomain "issueops/internal/domain/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
	cycleport "issueops/internal/port/issueopscycle"
)

func ImplementationReviewMissing(record model.IssueOpsRecord, currentFingerprint string) string {
	evidence := reviewcontract.ReviewGateEvidence{}
	if review := record.ImplementationReview; review != nil {
		evidence = reviewcontract.ReviewGateEvidence{
			Present: true, Verdict: review.Verdict, ReviewedFingerprint: review.ReviewedFingerprint,
		}
	}
	return reviewdomain.ImplementationReviewMissing(record.Execution != nil, evidence, currentFingerprint)
}

func ProjectDocsReviewMissing(record model.IssueOpsRecord, currentFingerprint string) string {
	evidence := reviewcontract.ReviewGateEvidence{}
	if review := record.ProjectDocsReview; review != nil {
		evidence = reviewcontract.ReviewGateEvidence{Present: true, ReviewedFingerprint: review.ReviewedFingerprint}
	}
	return reviewdomain.ProjectDocsReviewMissing(evidence, currentFingerprint)
}

func LocalPRReadinessMissing(record model.IssueOpsRecord, observations cycleport.ReadinessObservations) []string {
	missing := BaseImplementationMissing(record)
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
	if HasUnresolvedContractFeedback(record) {
		missing = append(missing, "contract_feedback_issue_update")
	}
	return missing
}
