package issueopsreview

import "strings"

func FeedbackRequiresIssueUpdate(classification, issueUpdatedAt string) bool {
	return strings.EqualFold(strings.TrimSpace(classification), "contract_change") &&
		strings.TrimSpace(issueUpdatedAt) == ""
}
