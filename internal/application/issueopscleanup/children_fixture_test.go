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
	Read       func(string, string) (model.IssueOpsRecord, error)
	TouchWrite func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error)
	Provider   func(string) (port.IssueProvider, error)
}

type childCleanupFixtureRecords struct {
	store Store
	root  string
}

func (r childCleanupFixtureRecords) WithinLock(_ context.Context, _ string, fn func() error) error {
	return fn()
}
func (r childCleanupFixtureRecords) Load(id string) (model.IssueOpsRecord, error) {
	return r.store.Read(r.root, id)
}
func (r childCleanupFixtureRecords) Save(record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
	return r.store.TouchWrite(r.root, record)
}
func ByID(store Store, root, id string, req model.IssueOpsCloseChildrenRequest) (model.IssueOpsCloseChildrenResult, error) {
	return (app.ChildrenCloser{Records: childCleanupFixtureRecords{store, root}, Provider: store.Provider, VerifyMerged: func(model.IssueOpsRemoteArtifactVerification) error {
		if req.Merged {
			return nil
		}
		return fmt.Errorf("not merged")
	}, Now: time.Now}).Close(context.Background(), id, req.Merged || req.MergeEvidenceRequested, req.Confirm)
}
