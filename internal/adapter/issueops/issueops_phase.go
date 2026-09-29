package issueops

import (
	"issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

func issueOpsPhaseRank(phase issueops.IssueOpsPhase) int { return domain.IssueOpsPhaseRank(phase) }
