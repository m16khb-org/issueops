package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	cycledomain "issueops/internal/domain/issueops"
	"issueops/internal/domain/stringlist"
)

func ApplyDuplicateGateLedger(ready model.IssueOpsReadiness, issueNumber string, canonicalExists bool, entries []cycledomain.GateLedgerFile) model.IssueOpsReadiness {
	missing := cycledomain.DuplicateGateLedgerMissing(issueNumber, canonicalExists, entries)
	if len(missing) == 0 {
		return ready
	}
	ready.Missing = stringlist.UniqueSorted(append(append([]string{}, ready.Missing...), missing...))
	ready.Ready = false
	return ready
}
