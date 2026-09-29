package issueops

import (
	"context"
	"issueops/internal/adapter/issueops/pathutil"
	"issueops/internal/domain/repoidentity"

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

type CycleStartIdentity struct {
	RunGit func(string, ...string) (int, string, string)
}

func (identity CycleStartIdentity) CanonicalRepo(repo string) string {
	clean := pathutil.CleanAbsPath(repo)
	if identity.RunGit == nil {
		return clean
	}
	code, commonDir, _ := identity.RunGit(clean, "rev-parse", "--path-format=relative", "--git-common-dir")
	if code != 0 {
		commonDir = ""
	}
	return repoidentity.SourceRoot(clean, commonDir)
}
func (CycleStartIdentity) StableID(repo, branch string) string { return newIssueOpsID(repo, branch) }
func (CycleStartIdentity) IndependentID(repo string) (string, error) {
	return newIndependentIssueOpsID(repo)
}
