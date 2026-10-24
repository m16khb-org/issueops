package issueops

import (
	"context"
	"errors"
	"io/fs"
	"strconv"

	"issueops/internal/adapter/issueops/pathutil"
	"issueops/internal/adapter/outbound/sqlstore"
	"issueops/internal/domain/repoidentity"

	model "issueops/internal/contract/issueops"
)

type CycleRecordStore struct{ StateRoot string }

func (s CycleRecordStore) WithinLock(ctx context.Context, id string, fn func(context.Context) error) error {
	return withIssueOpsLock(ctx, s.StateRoot, id, fn)
}

func (s CycleRecordStore) Load(id string) (model.IssueOpsRecord, error) {
	return ReadIssueOps(s.StateRoot, id)
}

func (s CycleRecordStore) Save(ctx context.Context, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
	return writeIssueOps(ctx, s.StateRoot, record)
}

// RetiredIDs names the cycle IDs whose records cleanup deleted: cleanup keeps
// one outcome row per deleted record under the same ID. An ID that also has a
// live record (a cycle that reused it before IDs were retired) stays
// resumable.
func (s CycleRecordStore) RetiredIDs() (map[string]bool, error) {
	ids, err := sqlstore.ListExisting(s.StateRoot, cycleOutcomeBucket)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	retired := make(map[string]bool, len(ids))
	for _, id := range ids {
		retired[id] = true
	}
	live, err := sqlstore.ListExisting(s.StateRoot, issueOpsBucket)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	for _, id := range live {
		delete(retired, id)
	}
	return retired, nil
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

// SuccessorID is the n-th stable ID for repo and branch; n=0 is StableID.
func (CycleStartIdentity) SuccessorID(repo, branch string, n int) string {
	if n == 0 {
		return newIssueOpsID(repo, branch)
	}
	return newIssueOpsID(repo, branch+"\x00"+strconv.Itoa(n))
}
func (CycleStartIdentity) IndependentID(repo string) (string, error) {
	return newIndependentIssueOpsID(repo)
}
