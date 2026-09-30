package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	cycledomain "issueops/internal/domain/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
)

func DevilsAdvocateStagedPlanBound(record model.IssueOpsRecord, stagedDigest string) bool {
	beforeImplement := cycledomain.IssueOpsPhaseRank(record.Phase) < cycledomain.IssueOpsPhaseRank(model.IssueOpsPhaseImplement)
	return reviewdomain.DevilsAdvocateStagedPlanBound(record.DevilsAdvocateReview, beforeImplement, stagedDigest)
}
