package issueopsreview

import "strings"

type FeedbackSnapshot struct {
	Classification string
	IssueUpdatedAt string
	Resolution     string
}

func HasUnresolvedContractFeedback(items []FeedbackSnapshot) bool {
	for _, item := range items {
		if FeedbackRequiresIssueUpdate(item.Classification, item.IssueUpdatedAt) {
			return true
		}
	}
	return false
}

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
