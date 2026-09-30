package issueops

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"issueops/internal/adapter/outbound/sqlstore"
	issueopsdomain "issueops/internal/domain/issueops"
)

// withIssueOpsLock serializes the full read-modify-write span for a cycle
// against every other span on the same state root, in-process and
// cross-process, via the sqlstore span lock (a held BEGIN IMMEDIATE
// transaction on the state root's lock database that dies with the process).
// The callback receives the active-root context, so same-root re-entry and
// cycles are rejected before waiting while documented distinct-root ordering
// remains possible.
func withIssueOpsLock(ctx context.Context, stateRoot, id string, fn func(context.Context) error) error {
	if _, err := normalizeIssueOpsID(id); err != nil {
		return err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return err
	}
	return db.WithSpan(ctx, func(spanCtx context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
			return readErr
		}
		if readErr == nil {
			if err := issueopsdomain.RequireNoCleanupAttempt(record.CleanupAttempt); err != nil {
				return err
			}
			if record.CleanupAbandonFailure != nil && record.CleanupAbandonFailure.Step == "applying" {
				return fmt.Errorf("cleanup abandon apply is in progress")
			}
		}
		return fn(spanCtx)
	})
}
