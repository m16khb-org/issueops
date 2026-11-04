package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
)

func feedbackCompletionMissing(record model.Record) []string {
	return reviewdomain.FeedbackCompletionMissing(feedbackSnapshots(record))
}

func hasUnresolvedContractFeedback(record model.Record) bool {
	return reviewdomain.HasUnresolvedContractFeedback(feedbackSnapshots(record))
}

func feedbackSnapshots(record model.Record) []reviewdomain.FeedbackSnapshot {
	items := make([]reviewdomain.FeedbackSnapshot, 0, len(record.Feedback))
	for _, item := range record.Feedback {
		items = append(items, reviewdomain.FeedbackSnapshot{
			Classification: item.Classification,
			IssueUpdatedAt: item.IssueUpdatedAt,
			Resolution:     item.Resolution,
		})
	}
	return items
}
