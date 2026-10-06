package issueops

import (
	reviewapp "issueops/internal/application/issueopsreview"
	"issueops/internal/contract/issueops"
)

func AddIssueOpsFeedback(stateRoot, id, source, body, classification string) (issueops.IssueOpsRecord, error) {
	return addIssueOpsFeedback(stateRoot, id, source, body, classification, nil)
}

func addIssueOpsFeedback(stateRoot, id, source, body, classification string, actor *issueops.IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return reviewapp.AddFeedback(NewReviewMutationStore(actor), stateRoot, id, source, body, classification)
}
