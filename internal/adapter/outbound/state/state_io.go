package state

import (
	"context"
	"issueops/internal/adapter/outbound/sqlstore"
	stateapplication "issueops/internal/application/state"
	statecontract "issueops/internal/contract/state"
	"issueops/internal/domain/statepath"
	stateport "issueops/internal/port/state"
)

const stateBucket = "state"

func openStateStore(dir string) (stateport.Store, error) {
	return sqlstore.Open(dir)
}

// ExistingRecords reads records without creating an absent store.
type ExistingRecords struct{}

// GetExisting returns the record id in bucket under dir without creating a store.
// The bool is false when the store or the record does not exist.
func (ExistingRecords) GetExisting(dir, bucket, id string) ([]byte, bool, error) {
	return sqlstore.GetExisting(dir, bucket, id)
}

var _ stateport.ExistingReader = ExistingRecords{}

// NewService assembles the state application service over the process state
// directory and the shared SQLite store.
func NewService() *stateapplication.Service {
	return stateapplication.NewService(stateapplication.Dependencies{
		StateDir:        Dir,
		StatePath:       statepath.Path,
		OpenStore:       openStateStore,
		ExistingRecords: ExistingRecords{},
	})
}

// WriteStateRecord writes record under key in the state store at dir and returns
// the stored record path.
func WriteStateRecord(ctx context.Context, dir, key string, record statecontract.RecordEnvelope) (string, error) {
	return NewService().WriteRecord(ctx, dir, key, record)
}
