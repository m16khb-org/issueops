package sqlstore

import (
	"context"

	"issueops/internal/adapter/outbound/processlease"
	statecontract "issueops/internal/contract/state"
)

// ExcludeWrites excludes every record mutation while allowing reads. It does
// not acquire a SQLite span or transaction. Callers must not write records or
// acquire a span while holding it; inherited children retain the exclusion.
func (d *DB) ExcludeWrites(ctx context.Context) (*processlease.Lease, error) {
	return processlease.Acquire(ctx, d.dir, statecontract.RecordWriteLeaseKey)
}
