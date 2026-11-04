package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

func ShouldRefreshAISlopClean(record model.Record, phase model.Phase) bool {
	return phase == model.PhaseAISlopClean &&
		strings.TrimSpace(record.AISlopCleanAt) != "" &&
		PhaseRank(record.Phase) > PhaseRank(model.PhaseAISlopClean)
}
