package issueopsbranch_test

import (
	"context"
	"path/filepath"
	"time"

	instructions "issueops/internal/adapter/issueops/branchinstructions"
	application "issueops/internal/application/issueopsbranch"
	model "issueops/internal/contract/issueops"
)

type Store struct {
	Read                  func(string, string) (model.IssueOpsRecord, error)
	TouchWrite            func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error)
	ResolveBaseCommit     func(string, string) (string, error)
	UmbrellaForChildIssue func(string, string) (model.IssueOpsRecord, bool)
	ObserveCodeProjectKey func(string, string) (string, error)
}
type preparationRepository struct {
	store  Store
	root   string
	locked bool
}

func (r *preparationRepository) WithinLock(ctx context.Context, _ string, fn func(context.Context) error) error {
	r.locked = true
	defer func() { r.locked = false }()
	return fn(ctx)
}
func (r *preparationRepository) Load(id string) (model.IssueOpsRecord, error) {
	if !r.locked {
		panic("read outside lock")
	}
	return r.store.Read(r.root, id)
}
func (r *preparationRepository) Save(_ context.Context, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
	if !r.locked {
		panic("write outside lock")
	}
	return r.store.TouchWrite(r.root, record)
}
func prepareForTest(store Store, root, id string, req model.IssueOpsBranchPrepareRequest) (model.IssueOpsRecord, error) {
	service := application.Preparer{Records: &preparationRepository{store: store, root: root}, CleanParentPath: func(path string) (string, bool) { return filepath.Clean(path), filepath.IsAbs(path) }, ResolveBaseCommit: store.ResolveBaseCommit, UmbrellaForChildIssue: store.UmbrellaForChildIssue, ObserveCodeProjectKey: store.ObserveCodeProjectKey, Steps: instructions.Steps, Now: time.Now}
	return service.Prepare(context.Background(), id, req, nil)
}
