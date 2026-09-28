package issueopsapp

import (
	"context"
	"time"

	"issueops/internal/adapter/issueops"
	application "issueops/internal/application/issueopsremote"
	model "issueops/internal/contract/issueops"
)

func newIssueCreateIntents(stateRoot string) *application.IssueCreateIntents {
	return application.NewIssueCreateIntents(issueops.IssueCreateIntentStore{StateRoot: stateRoot}, time.Now)
}

func beginIssueCreateIntent(stateRoot, id string, request model.IssueOpsIssueCreateIntentRequest) (model.IssueOpsRecord, error) {
	return newIssueCreateIntents(stateRoot).Begin(context.Background(), id, request)
}

func recordIssueCreateOutcome(stateRoot, id string, outcome model.IssueOpsIssueCreateOutcome) (model.IssueOpsRecord, error) {
	return newIssueCreateIntents(stateRoot).Outcome(context.Background(), id, outcome)
}

func completeIssueCreateIntent(stateRoot, id, issueURL, completedAt string) (model.IssueOpsRecord, error) {
	return newIssueCreateIntents(stateRoot).Complete(context.Background(), id, issueURL, completedAt)
}
