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

func StateDir() string {
	return stateDir()
}

func NormalizeStateKey(key string) (string, error) {
	return statepath.NormalizeKey(key)
}

func statePath(dir, key string) string {
	return statepath.Path(dir, key)
}

func openStateDB(dir string) (*sqlstore.DB, error) {
	return sqlstore.Open(dir)
}

func openStateStore(dir string) (stateport.Store, error) {
	return sqlstore.Open(dir)
}

// ExistingRecords reads records without creating an absent store.
type ExistingRecords struct{}

func (ExistingRecords) GetExisting(dir, bucket, id string) ([]byte, bool, error) {
	return sqlstore.GetExisting(dir, bucket, id)
}

var _ stateport.ExistingReader = ExistingRecords{}

func service() *stateapplication.Service {
	return stateapplication.NewService(stateapplication.Dependencies{
		StateDir:        stateDir,
		StatePath:       statepath.Path,
		OpenStore:       openStateStore,
		ExistingRecords: ExistingRecords{},
	})
}

func StateWrite(ctx context.Context, key, content string) (statecontract.StateResult, error) {
	return service().Write(ctx, key, content)
}

func StateRead(key string) (statecontract.StateResult, error) {
	return service().Read(key)
}

func StateList() (statecontract.StateListResult, error) {
	return service().List()
}

func WriteStateRecord(ctx context.Context, dir, key string, record statecontract.RecordEnvelope) (string, error) {
	return service().WriteRecord(ctx, dir, key, record)
}

func StateDelete(ctx context.Context, key string) error {
	return service().Delete(ctx, key)
}
