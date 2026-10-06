package issueops

import (
	"path/filepath"
	"strings"
)

// ScopeGateLedgers judges the linked issue's ledger and anonymous ledgers.
// Without an issue number it judges every discovered ledger; numeric issue
// folders belonging to another issue are skipped.
func ScopeGateLedgers(root string, files []string, issueNumber string) (judged, skipped []string) {
	issueNumber = strings.TrimSpace(issueNumber)
	if issueNumber == "" {
		return files, nil
	}
	issuesDir := filepath.Join(root, ".issueops", "issues") + string(filepath.Separator)
	for _, file := range files {
		if strings.HasPrefix(file, issuesDir) {
			folder := strings.SplitN(strings.TrimPrefix(file, issuesDir), string(filepath.Separator), 2)[0]
			if folder != issueNumber && gateLedgerAllDigits(folder) {
				skipped = append(skipped, file)
				continue
			}
		}
		judged = append(judged, file)
	}
	return judged, skipped
}

func gateLedgerAllDigits(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
