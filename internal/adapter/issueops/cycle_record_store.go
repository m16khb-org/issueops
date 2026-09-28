package issueops

import (
	"context"

	model "issueops/internal/contract/issueops"
)

type CycleRecordStore struct{ StateRoot string }

func (s CycleRecordStore) WithinLock(ctx context.Context, id string, fn func() error) error {
	return withIssueOpsLock(ctx, s.StateRoot, id, func(context.Context) error { return fn() })
}

func (s CycleRecordStore) Load(id string) (model.IssueOpsRecord, error) {
	return ReadIssueOps(s.StateRoot, id)
}

func (s CycleRecordStore) Save(record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
	return writeIssueOps(s.StateRoot, record)
}

type CycleStartIdentity struct{}

func (CycleStartIdentity) CanonicalRepo(repo string) string    { return normalizeIssueOpsRepo(repo) }
func (CycleStartIdentity) StableID(repo, branch string) string { return newIssueOpsID(repo, branch) }
func (CycleStartIdentity) IndependentID(repo string) (string, error) {
	return newIndependentIssueOpsID(repo)
}
