package issueops

import (
	"context"
	"time"

	reviewapp "issueops/internal/application/issueopsreview"
	"issueops/internal/contract/issueops"
	reviewport "issueops/internal/port/issueopsreview"
)

func AddIssueOpsFeedback(stateRoot, id, source, body, classification string) (issueops.IssueOpsRecord, error) {
	return addIssueOpsFeedback(stateRoot, id, source, body, classification, nil)
}

func AddIssueOpsFeedbackWithActor(stateRoot, id, source, body, classification string, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return addIssueOpsFeedback(stateRoot, id, source, body, classification, &actor)
}

func addIssueOpsFeedback(stateRoot, id, source, body, classification string, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return reviewapp.AddFeedback(reviewMutationStore(actor), stateRoot, id, source, body, classification)
}

func MarkIssueOpsContractFeedbackIssueUpdatedWithActor(stateRoot, id string, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return markIssueOpsContractFeedbackIssueUpdated(stateRoot, id, &actor)
}

func markIssueOpsContractFeedbackIssueUpdated(stateRoot, id string, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return reviewapp.MarkContractFeedbackIssueUpdated(reviewMutationStore(actor), stateRoot, id)
}

func reviewMutationStore(actor *IssueOpsActor) reviewport.ReviewMutationStore {
	return reviewport.ReviewMutationStore{
		WithLock: func(root, cycleID string, fn func() error) error {
			return withIssueOpsLock(context.Background(), root, cycleID, func(context.Context) error { return fn() })
		},
		Read: ReadIssueOps,
		ValidateMutation: func(record issueops.IssueOpsRecord) error {
			return validatePostTransferMutation(record, actor)
		},
		Write: writeIssueOps,
		Now:   func() string { return time.Now().UTC().Format(time.RFC3339Nano) },
	}
}
