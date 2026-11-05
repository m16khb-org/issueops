package issueops

import (
	reviewapp "issueops/internal/application/issueopsreview"
	"issueops/internal/contract/issueops"
)

func AddIssueOpsFeedback(stateRoot, id, source, body, classification string) (issueops.Record, error) {
	return addIssueOpsFeedback(stateRoot, id, source, body, classification, nil)
}

func addIssueOpsFeedback(stateRoot, id, source, body, classification string, actor *issueops.Actor) (issueops.Record, error) {
	return reviewapp.AddFeedback(NewReviewMutationStore(actor), stateRoot, id, source, body, classification)
}
