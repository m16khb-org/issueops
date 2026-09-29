package selfverify

import (
	contract "issueops/internal/contract/selfaugment"
	domain "issueops/internal/domain/selfaugment"
	"time"
)

type ExportCandidatesDeps struct {
	Source func(root string) (path string, exists bool)
	Now    func() time.Time
}

func ExportCandidates(root string, deps ExportCandidatesDeps) contract.SelfVerificationCandidateExportResult {
	path, exists := deps.Source(root)
	result := domain.NewCandidateExport(exists)
	result.SourcePath = path
	result.IssueOpsRoot = root
	result.GeneratedAt = deps.Now().UTC().Format(time.RFC3339Nano)
	return result
}
