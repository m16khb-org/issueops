package issueopsreview

import "strings"

// FeedbackSnapshot is the subset of a feedback item that completion checks read.
type FeedbackSnapshot struct {
	Classification string
	IssueUpdatedAt string
	Resolution     string
}

// HasUnresolvedContractFeedback reports whether any contract_change feedback item
// still lacks an issue update.
func HasUnresolvedContractFeedback(items []FeedbackSnapshot) bool {
	for _, item := range items {
		if FeedbackRequiresIssueUpdate(item.Classification, item.IssueUpdatedAt) {
			return true
		}
	}
	return false
}

// FeedbackCompletionMissing returns the keys that block completion: an unclassified
// item, unresolved contract feedback, and an item without a resolution. Each key
// appears at most once.
func FeedbackCompletionMissing(items []FeedbackSnapshot) []string {
	missing := []string{}
	for _, item := range items {
		if strings.TrimSpace(item.Classification) == "" {
			missing = append(missing, "feedback_classification")
			break
		}
	}
	if HasUnresolvedContractFeedback(items) {
		missing = append(missing, "contract_feedback_issue_update")
	}
	for _, item := range items {
		if strings.TrimSpace(item.Resolution) == "" {
			missing = append(missing, "feedback_resolution")
			break
		}
	}
	return missing
}
