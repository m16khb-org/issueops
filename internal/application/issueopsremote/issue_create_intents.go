package issueopsremote

import (
	"context"
	"fmt"
	"strings"
	"time"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/domain/issueopsremote"
)

// IssueCreateIntents 타입은 이슈 생성 의도(시작, 결과, 완료)를 기록에 durable하게 남기고 갱신 시각을 찍는다.
type IssueCreateIntents struct {
	store RecordStore
	now   func() time.Time
}

// NewIssueCreateIntents 함수는 저장소와 시계로 IssueCreateIntents를 만든다.
func NewIssueCreateIntents(store RecordStore, now func() time.Time) *IssueCreateIntents {
	return &IssueCreateIntents{store: store, now: now}
}

// Begin 메서드는 요청을 봉인한 생성 의도를 기록에 시작으로 남긴다.
func (s *IssueCreateIntents) Begin(ctx context.Context, id string, request model.IssueCreateIntentRequest) (model.Record, error) {
	return s.update(ctx, id, func(record model.Record) (model.Record, error) {
		return domain.BeginIssueCreateIntent(record, request)
	})
}

// Outcome 메서드는 생성 시도의 결과(실패 상태, 정본 URL, 사유)를 의도에 기록한다.
func (s *IssueCreateIntents) Outcome(ctx context.Context, id string, outcome model.IssueCreateOutcome) (model.Record, error) {
	return s.update(ctx, id, func(record model.Record) (model.Record, error) {
		return domain.RecordIssueCreateOutcome(record, outcome, issueCreateProject(record, outcome.CanonicalURL))
	})
}

// releaseCrashed records not_invoked for the exact pending attempt that was
// settled: a concurrent reconcile or a new create-issue attempt changes the
// attempt or its timestamp, and the release then refuses instead of
// overwriting the newer attempt (security review 2026-10-25).
func (s *IssueCreateIntents) releaseCrashed(ctx context.Context, id string, settled model.IssueCreateIntent, outcome model.IssueCreateOutcome) (model.Record, error) {
	return s.update(ctx, id, func(record model.Record) (model.Record, error) {
		current := record.IssueCreateIntent
		if current == nil || current.Status != model.IssueCreateIntentPending || current.Attempt != settled.Attempt ||
			current.UpdatedAt != settled.UpdatedAt || current.OperationID != settled.OperationID {
			return record, fmt.Errorf("issue create intent changed since it was settled; run reconcile-issue again")
		}
		return domain.RecordIssueCreateOutcome(record, outcome, issueCreateProject(record, outcome.CanonicalURL))
	})
}

// Complete 메서드는 생성된 이슈 URL로 의도를 완료 처리한다.
func (s *IssueCreateIntents) Complete(ctx context.Context, id, issueURL, completedAt string) (model.Record, error) {
	return s.update(ctx, id, func(record model.Record) (model.Record, error) {
		return domain.CompleteIssueCreateIntent(record, issueURL, completedAt, issueCreateProject(record, issueURL))
	})
}

func (s *IssueCreateIntents) update(ctx context.Context, id string, transition RecordTransition) (model.Record, error) {
	return s.store.Update(ctx, id, func(record model.Record) (model.Record, error) {
		updated, err := transition(record)
		if err != nil {
			return updated, err
		}
		updated.UpdatedAt = s.now().UTC().Format(time.RFC3339Nano)
		return updated, nil
	})
}

func issueCreateProject(record model.Record, issueURL string) string {
	if record.IssueCreateIntent == nil || strings.TrimSpace(issueURL) == "" {
		return ""
	}
	return issueopsremote.ProjectKey(strings.TrimSpace(issueURL), record.IssueCreateIntent.Provider, "issue")
}
