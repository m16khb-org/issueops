package issueopsapp

import (
	"context"

	looprunadapter "issueops/internal/adapter/looprun"
	"issueops/internal/adapter/outbound/sqlstore"
	workeradapter "issueops/internal/adapter/worker"
)

// grantFencedLoopStore holds the IssueOps grant-root span around each loop
// span. A capability-bound request's record guard binds in that outer span, so
// the grant is rechecked under the same lock rotation takes; a rotation that
// committed while the request waited rejects the loop write.
type grantFencedLoopStore struct {
	looprunadapter.Store
	grantRoot string
}

func (s grantFencedLoopStore) WithLock(ctx context.Context, loopID string, fn func(context.Context) error) error {
	grants, err := sqlstore.Open(s.grantRoot)
	if err != nil {
		return err
	}
	return grants.WithSpan(ctx, func(fenced context.Context) error {
		return s.Store.WithLock(fenced, loopID, fn)
	})
}

// grantFencedWorkerStore does the same for worker jobs. Jobs live in the fixed
// user-level worker store, but the workspace command they run was authorized by
// a capability, so that grant is rechecked under the grant lock before the
// worker span opens; a revocation committed meanwhile rejects the run.
type grantFencedWorkerStore struct {
	workeradapter.Store
	grantRoot string
}

func (s grantFencedWorkerStore) WithLock(ctx context.Context, dir, id string, fn func(context.Context) error) error {
	grants, err := sqlstore.Open(s.grantRoot)
	if err != nil {
		return err
	}
	return grants.WithSpan(ctx, func(fenced context.Context) error {
		return s.Store.WithLock(fenced, dir, id, fn)
	})
}
