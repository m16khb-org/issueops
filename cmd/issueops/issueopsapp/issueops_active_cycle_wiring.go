package issueopsapp

import (
	core "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/issueops/pathutil"
	branchapp "issueops/internal/application/issueopsbranch"
	model "issueops/internal/contract/issueops"
)

func newActiveCycleReader(root string) branchapp.ActiveCycleReader {
	return branchapp.ActiveCycleReader{
		Scan:      func() ([]model.IssueOpsRecord, error) { return core.ScanReadableIssueOps(root) },
		CleanPath: pathutil.CleanAbsPath,
	}
}
