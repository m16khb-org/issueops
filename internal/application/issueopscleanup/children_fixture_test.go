package issueopscleanup_test

import (
	"context"
	"fmt"
	"time"

	app "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// Keep the original provider scenarios on the new application use case.
type Store struct {
	Read       func(string, string) (model.Record, error)
	TouchWrite func(string, model.Record) (model.Record, error)
	Provider   func(string) (port.IssueProvider, error)
}

type childCleanupFixtureRecords struct {
	store Store
	root  string
}

func (r childCleanupFixtureRecords) WithinLock(ctx context.Context, _ string, fn func(context.Context) error) error {
	return fn(ctx)
}
func (r childCleanupFixtureRecords) Load(id string) (model.Record, error) {
	return r.store.Read(r.root, id)
}
func (r childCleanupFixtureRecords) Save(_ context.Context, record model.Record) (model.Record, error) {
	return r.store.TouchWrite(r.root, record)
}
func ByID(store Store, root, id string, req model.CloseChildrenRequest) (model.CloseChildrenResult, error) {
	return (app.ChildrenCloser{Records: childCleanupFixtureRecords{store, root}, Provider: store.Provider, VerifyMerged: func(model.RemoteArtifactVerification) error {
		if req.Merged {
			return nil
		}
		return fmt.Errorf("not merged")
	}, Now: time.Now}).Close(context.Background(), id, req.Merged || req.MergeEvidenceRequested, req.Confirm)
}
