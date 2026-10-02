package issueops

import (
	"context"
	"time"

	"issueops/internal/contract/issueops"
	reviewport "issueops/internal/port/issueopsreview"
)

func NewReviewMutationStore(actor *issueops.IssueOpsActor) reviewport.ReviewMutationStore {
	return reviewport.ReviewMutationStore{
		WithLock: func(root, cycleID string, fn func() error) error {
			return withIssueOpsLock(context.Background(), root, cycleID, func(context.Context) error { return fn() })
		},
		Read: ReadIssueOps,
		ValidateMutation: func(record issueops.IssueOpsRecord) error {
			return validatePostTransferMutation(context.Background(), record, actor, NativeActorVerifier())
		},
		Write: func(root string, record issueops.IssueOpsRecord) (issueops.IssueOpsRecord, error) {
			return writeIssueOps(context.Background(), root, record)
		},
		Now: func() string { return time.Now().UTC().Format(time.RFC3339Nano) },
	}
}
