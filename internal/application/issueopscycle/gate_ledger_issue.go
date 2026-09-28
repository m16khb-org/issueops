package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	issueopsremote "issueops/internal/domain/issueopsremote"
)

func GateLedgerIssueNumber(record model.IssueOpsRecord) string {
	if number := issueopsremote.IssueNumber(record.IssueURL); number != "" {
		return number
	}
	if record.BranchPrepare != nil {
		return issueopsremote.IssueNumber(record.BranchPrepare.IssueURL)
	}
	return ""
}
