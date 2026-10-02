package issueopscleanup

import (
	"context"
	"time"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

type LinkedBranchAuditRecords interface {
	WithinLock(context.Context, string, func(context.Context) error) error
	Load(string) (model.IssueOpsRecord, error)
	Save(context.Context, model.IssueOpsRecord) (model.IssueOpsRecord, error)
}

type LinkedBranchAuditRecorder struct {
	Records LinkedBranchAuditRecords
	Now     func() time.Time
}

// Record is best-effort: an audit failure cannot undo the provider's disposition.
// Reload under the cycle lock so the observation never recreates a deleted cycle
// or overwrites a concurrent mutation with its earlier snapshot.
func (s LinkedBranchAuditRecorder) Record(ctx context.Context, observed model.IssueOpsRecord, result model.CleanupLinkedBranchResult) (bool, string) {
	err := s.Records.WithinLock(ctx, observed.ID, func(spanCtx context.Context) error {
		current, err := s.Records.Load(observed.ID)
		if err != nil {
			return err
		}
		updated, err := domain.ApplyLinkedBranchCleanupAudit(observed, current, result, s.Now().UTC().Format(time.RFC3339))
		if err != nil {
			return err
		}
		_, err = s.Records.Save(spanCtx, updated)
		return err
	})
	if err != nil {
		return false, err.Error()
	}
	return true, ""
}
