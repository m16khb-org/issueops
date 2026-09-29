package issueops

import (
	model "issueops/internal/contract/issueops"
)

func ApplyDuplicateGateLedger(ready model.IssueOpsReadiness, issueNumber string, canonicalExists bool, entries []GateLedgerFile) model.IssueOpsReadiness {
	missing := DuplicateGateLedgerMissing(issueNumber, canonicalExists, entries)
	if len(missing) == 0 {
		return ready
	}
	ready.Missing = gateMissingKeys(append(append([]string{}, ready.Missing...), missing...))
	ready.Ready = false
	return ready
}
