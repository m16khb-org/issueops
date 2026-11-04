package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

// CleanupCommandGeneration returns the execution lease generation that a
// cleanup command must be issued under, or 0 when the record has no
// execution.
func CleanupCommandGeneration(record model.Record) uint64 {
	if record.Execution == nil {
		return 0
	}
	return record.Execution.Lease.Generation
}

// CleanupCommandNeedsProvenance reports whether a cleanup command requires
// lease provenance: it is non-blank and tied to a non-zero lease generation.
func CleanupCommandNeedsProvenance(command string, generation uint64) bool {
	return strings.TrimSpace(command) != "" && generation != 0
}
