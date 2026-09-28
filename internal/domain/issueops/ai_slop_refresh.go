package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

func ShouldRefreshAISlopClean(record model.IssueOpsRecord, phase model.IssueOpsPhase) bool {
	return phase == model.IssueOpsPhaseAISlopClean &&
		strings.TrimSpace(record.AISlopCleanAt) != "" &&
		IssueOpsPhaseRank(record.Phase) > IssueOpsPhaseRank(model.IssueOpsPhaseAISlopClean)
}
