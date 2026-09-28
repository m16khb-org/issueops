package issueopsremote

import (
	"context"
	"strings"
	"time"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	remote "issueops/internal/domain/issueopsremote"
)

type IssueCreateIntents struct {
	store IssueIntentStore
	now   func() time.Time
}

func NewIssueCreateIntents(store IssueIntentStore, now func() time.Time) *IssueCreateIntents {
	return &IssueCreateIntents{store: store, now: now}
}

func (s *IssueCreateIntents) Begin(ctx context.Context, id string, request model.IssueOpsIssueCreateIntentRequest) (model.IssueOpsRecord, error) {
	return s.update(ctx, id, func(record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
		return domain.BeginIssueCreateIntent(record, request)
	})
}

func (s *IssueCreateIntents) Outcome(ctx context.Context, id string, outcome model.IssueOpsIssueCreateOutcome) (model.IssueOpsRecord, error) {
	return s.update(ctx, id, func(record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
		return domain.RecordIssueCreateOutcome(record, outcome, issueCreateProject(record, outcome.CanonicalURL))
	})
}

func (s *IssueCreateIntents) Complete(ctx context.Context, id, issueURL, completedAt string) (model.IssueOpsRecord, error) {
	return s.update(ctx, id, func(record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
		return domain.CompleteIssueCreateIntent(record, issueURL, completedAt, issueCreateProject(record, issueURL))
	})
}

func (s *IssueCreateIntents) update(ctx context.Context, id string, transition IssueIntentTransition) (model.IssueOpsRecord, error) {
	return s.store.Update(ctx, id, func(record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
		updated, err := transition(record)
		if err != nil {
			return updated, err
		}
		updated.UpdatedAt = s.now().UTC().Format(time.RFC3339Nano)
		return updated, nil
	})
}

func issueCreateProject(record model.IssueOpsRecord, issueURL string) string {
	if record.IssueCreateIntent == nil || strings.TrimSpace(issueURL) == "" {
		return ""
	}
	return remote.ProjectKey(strings.TrimSpace(issueURL), record.IssueCreateIntent.Provider, "issue")
}
