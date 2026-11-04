package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

// ShouldRefreshAISlopClean reports whether recording the ai-slop-clean phase
// again refreshes an existing clean: the clean was already recorded and the
// record has moved past that phase.
func ShouldRefreshAISlopClean(record model.Record, phase model.Phase) bool {
	return phase == model.PhaseAISlopClean &&
		strings.TrimSpace(record.AISlopCleanAt) != "" &&
		PhaseRank(record.Phase) > PhaseRank(model.PhaseAISlopClean)
}
