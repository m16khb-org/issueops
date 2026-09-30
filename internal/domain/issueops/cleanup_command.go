package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

func CleanupCommandGeneration(record model.IssueOpsRecord) uint64 {
	if record.Execution == nil {
		return 0
	}
	return record.Execution.Lease.Generation
}
func CleanupCommandNeedsProvenance(command string, generation uint64) bool {
	return strings.TrimSpace(command) != "" && generation != 0
}
