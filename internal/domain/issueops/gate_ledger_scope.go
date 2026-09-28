package issueops

import (
	"path/filepath"
	"strings"
)

const GateLedgerCompatibilitySchemaVersion = 1

// ScopeGateLedgers judges the linked issue's ledger and anonymous ledgers.
// Without an issue number it judges every discovered ledger; numeric folders
// and legacy filename prefixes belonging to another issue are skipped.
func ScopeGateLedgers(root string, files []string, issueNumber string) (judged, skipped []string) {
	issueNumber = strings.TrimSpace(issueNumber)
	if issueNumber == "" {
		return files, nil
	}
	issuesDir := filepath.Join(root, ".issueops", "issues") + string(filepath.Separator)
	legacyDir := filepath.Join(root, ".issueops", "gates") + string(filepath.Separator)
	for _, file := range files {
		switch {
		case strings.HasPrefix(file, issuesDir):
			folder := strings.SplitN(strings.TrimPrefix(file, issuesDir), string(filepath.Separator), 2)[0]
			if folder != issueNumber && gateLedgerAllDigits(folder) {
				skipped = append(skipped, file)
				continue
			}
		case strings.HasPrefix(file, legacyDir):
			if number := LegacyGateLedgerIssueNumber(filepath.Base(file), GateLedgerCompatibilitySchemaVersion); number != "" && number != issueNumber {
				skipped = append(skipped, file)
				continue
			}
		}
		judged = append(judged, file)
	}
	return judged, skipped
}

// LegacyGateLedgerIssueNumber applies only to schema v1 compatibility paths.
func LegacyGateLedgerIssueNumber(name string, schemaVersion int) string {
	if schemaVersion != GateLedgerCompatibilitySchemaVersion {
		return ""
	}
	name = strings.TrimSuffix(name, ".md")
	name = strings.TrimPrefix(name, "issue-")
	digits := 0
	for digits < len(name) && name[digits] >= '0' && name[digits] <= '9' {
		digits++
	}
	if digits == 0 {
		return ""
	}
	if digits == len(name) || name[digits] == '-' {
		return name[:digits]
	}
	return ""
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
