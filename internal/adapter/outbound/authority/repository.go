package authority

import (
	"context"
	"errors"
	"io/fs"

	"issueops/internal/adapter/outbound/sqlstore"
	contract "issueops/internal/contract/authority"
	domain "issueops/internal/domain/authority"
	"issueops/internal/port"
	authorityport "issueops/internal/port/authority"
)

// Repository stores grants in the fixed user-state IssueOps database, the same
// root whose record spans the authority guard rechecks.
type Repository struct{ StateRoot string }

var (
	_ authorityport.Repository   = Repository{}
	_ authorityport.RecordReader = Repository{}
)

func (r Repository) Within(ctx context.Context, key string, fn func(*contract.Record) (*contract.Record, error)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	database, err := sqlstore.Open(r.StateRoot)
	if err != nil {
		return err
	}
	return database.WithSpan(ctx, func(spanCtx context.Context) error {
		data, found, err := database.Get(contract.Bucket, key)
		if err != nil {
			return err
		}
		var current *contract.Record
		if found {
			record, err := domain.DecodeRecord(data, key)
			if err != nil {
				return err
			}
			current = &record
		}
		next, err := fn(current)
		if err != nil || next == nil {
			return err
		}
		encoded, err := domain.EncodeRecord(*next)
		if err != nil {
			return err
		}
		return database.Apply(spanCtx, []port.RecordMutation{{Bucket: contract.Bucket, ID: key, Data: encoded}})
	})
}

// Get reads committed grants without creating state, for checks outside a span.
func (r Repository) Get(bucket, id string) ([]byte, bool, error) {
	data, found, err := sqlstore.GetExisting(r.StateRoot, bucket, id)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	return data, found, err
}
