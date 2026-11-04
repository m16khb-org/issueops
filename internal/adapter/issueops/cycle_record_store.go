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

// CycleRecordStore reads and writes cycle records in the SQLite state under StateRoot.
type CycleRecordStore struct{ StateRoot string }

// WithinLock runs fn while holding the per-cycle lock for id.
func (s CycleRecordStore) WithinLock(ctx context.Context, id string, fn func(context.Context) error) error {
	return withIssueOpsLock(ctx, s.StateRoot, id, fn)
}

// Load reads the cycle record for id.
func (s CycleRecordStore) Load(id string) (model.Record, error) {
	return ReadIssueOps(s.StateRoot, id)
}

// Save writes record with a compare-and-apply against the currently stored row and returns
// the stored form. A record that carries a cleanup attempt is refused.
func (s CycleRecordStore) Save(ctx context.Context, record model.Record) (model.Record, error) {
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

// Done reports a live record already in the done phase; a missing or
// unreadable record is not done (start then decides under the lock).
func (s CycleRecordStore) Done(id string) bool {
	record, err := ReadIssueOps(s.StateRoot, id)
	return err == nil && record.Phase == model.PhaseDone
}

// CycleStartIdentity derives cycle identities at start. RunGit receives the directory and
// git arguments and returns exit code, stdout and stderr; nil disables git lookups.
type CycleStartIdentity struct {
	RunGit func(string, ...string) (int, string, string)
}

// CanonicalRepo maps repo to the source repository root, so every linked worktree of one
// repository yields the same path. Without git or on failure it returns the cleaned absolute path.
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

// StableID returns the deterministic cycle ID for repo and branch.
func (CycleStartIdentity) StableID(repo, branch string) string { return newIssueOpsID(repo, branch) }

// SuccessorID returns the n-th stable ID for repo and branch; n=0 is StableID.
func (CycleStartIdentity) SuccessorID(repo, branch string, n int) string {
	if n == 0 {
		return newIssueOpsID(repo, branch)
	}
	return newIssueOpsID(repo, branch+"\x00"+strconv.Itoa(n))
}

// IndependentID returns a fresh cycle ID for repo that is not derived from a branch.
func (CycleStartIdentity) IndependentID(repo string) (string, error) {
	return newIndependentIssueOpsID(repo)
}
