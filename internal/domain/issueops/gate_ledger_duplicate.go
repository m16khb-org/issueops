package issueops

import "strings"

type GateLedgerFile struct {
	Name      string
	Directory bool
}

func DuplicateGateLedgerMissing(issueNumber string, canonicalExists bool, legacyEntries []GateLedgerFile) []string {
	issueNumber = strings.TrimSpace(issueNumber)
	if issueNumber == "" || !canonicalExists {
		return nil
	}
	for _, entry := range legacyEntries {
		if entry.Directory || !strings.HasSuffix(entry.Name, ".md") {
			continue
		}
		if LegacyGateLedgerIssueNumber(entry.Name, GateLedgerCompatibilitySchemaVersion) == issueNumber {
			return []string{"duplicate_issue_artifact:" + issueNumber}
		}
	}
	return nil
}

func DuplicateGateLedgerProbe(root, number string) (string, string, bool) {
	root, number = strings.TrimSpace(root), strings.TrimSpace(number)
	return root, number, root != "" && number != ""
}
