//go:build !unix

package sqlstore

import "context"

// Exclusive process leases are unsupported here, so no cleanup can obtain the
// exclusion. Ordinary SQLite writes retain their existing platform support.
func (d *DB) acquireRecordWriter(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return func() {}, nil
}
