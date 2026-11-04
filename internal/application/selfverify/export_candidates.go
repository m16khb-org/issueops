package selfverify

import (
	contract "issueops/internal/contract/selfaugment"
	domain "issueops/internal/domain/selfaugment"
	"time"
)

// ExportCandidatesDeps injects the candidate source lookup and the clock for ExportCandidates.
type ExportCandidatesDeps struct {
	Source func(root string) (path string, exists bool)
	Now    func() time.Time
}

// ExportCandidates builds the candidate export for root, recording where the source catalog was
// looked up, the root, and a UTC generation time.
func ExportCandidates(root string, deps ExportCandidatesDeps) contract.SelfVerificationCandidateExportResult {
	path, exists := deps.Source(root)
	result := domain.NewCandidateExport(exists)
	result.SourcePath = path
	result.IssueOpsRoot = root
	result.GeneratedAt = deps.Now().UTC().Format(time.RFC3339Nano)
	return result
}
