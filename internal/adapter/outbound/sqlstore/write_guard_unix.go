//go:build unix

package sqlstore

import (
	"context"
	"issueops/internal/adapter/outbound/processlease"
	statecontract "issueops/internal/contract/state"
)

func (d *DB) acquireRecordWriter(ctx context.Context) (func(), error) {
	lease, err := processlease.AcquireShared(ctx, d.dir, statecontract.RecordWriteLeaseKey)
	if err != nil {
		return nil, err
	}
	return func() { _ = lease.Close() }, nil
}
