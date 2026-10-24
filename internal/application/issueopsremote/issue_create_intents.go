package issueopsremote

import (
	"context"
	"fmt"
	"strings"
	"time"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	remote "issueops/internal/domain/issueopsremote"
)

type IssueCreateIntents struct {
	store RecordStore
	now   func() time.Time
}

func NewIssueCreateIntents(store RecordStore, now func() time.Time) *IssueCreateIntents {
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

// ReleaseCrashed records not_invoked for the exact pending attempt that was
// settled: a concurrent reconcile or a new create-issue attempt changes the
// attempt or its timestamp, and the release then refuses instead of
// overwriting the newer attempt (security review 2026-10-25).
func (s *IssueCreateIntents) ReleaseCrashed(ctx context.Context, id string, settled model.IssueOpsIssueCreateIntent, outcome model.IssueOpsIssueCreateOutcome) (model.IssueOpsRecord, error) {
	return s.update(ctx, id, func(record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
		current := record.IssueCreateIntent
		if current == nil || current.Status != model.IssueCreateIntentPending || current.Attempt != settled.Attempt ||
			current.UpdatedAt != settled.UpdatedAt || current.OperationID != settled.OperationID {
			return record, fmt.Errorf("issue create intent changed since it was settled; run reconcile-issue again")
		}
		return domain.RecordIssueCreateOutcome(record, outcome, issueCreateProject(record, outcome.CanonicalURL))
	})
}

func (s *IssueCreateIntents) Complete(ctx context.Context, id, issueURL, completedAt string) (model.IssueOpsRecord, error) {
	return s.update(ctx, id, func(record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
		return domain.CompleteIssueCreateIntent(record, issueURL, completedAt, issueCreateProject(record, issueURL))
	})
}

func (s *IssueCreateIntents) update(ctx context.Context, id string, transition RecordTransition) (model.IssueOpsRecord, error) {
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
