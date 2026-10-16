package issueopspreparation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	preparationapp "issueops/internal/application/issueopspreparation"
	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	"issueops/internal/port"
)

// getErrStore fails reads of one bucket so repository load errors surface.
type getErrStore struct {
	*preparationStore
	err error
}

func (store getErrStore) Get(bucket, id string) ([]byte, bool, error) {
	if bucket == recordBucket {
		return nil, false, store.err
	}
	return store.preparationStore.Get(bucket, id)
}

// preparedRepositoryFixture seeds a record and commits its direct preparation,
// so the record carries an execution workspace root.
func preparedRepositoryFixture(t *testing.T) (*preparationStore, *SQLiteRepository, preparationapp.DirectCommit) {
	t.Helper()
	store := newPreparationStore()
	record := repositoryRecord("io-prepared", "/repo", "199-prepared")
	store.seedRecord(t, record)
	repository := NewSQLiteRepository(store)
	snapshot, err := repository.Load(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	commit := directRepositoryCommit(snapshot)
	if _, err := repository.CommitDirect(context.Background(), commit); err != nil {
		t.Fatal(err)
	}
	return store, repository, commit
}

func TestRepositoryLoadFailsClosed(t *testing.T) {
	ctx := context.Background()
	if _, err := NewSQLiteRepository(nil).Load(ctx, "io-1"); err == nil || !strings.Contains(err.Error(), "store is unavailable") {
		t.Fatalf("nil store: %v", err)
	}
	store := newPreparationStore()
	repository := NewSQLiteRepository(store)
	if _, err := repository.Load(ctx, "io-missing"); err == nil || !strings.Contains(err.Error(), "io-missing not found") {
		t.Fatalf("missing record: %v", err)
	}
	boom := errors.New("disk read failed")
	if _, err := NewSQLiteRepository(getErrStore{store, boom}).Load(ctx, "io-1"); !errors.Is(err, boom) {
		t.Fatalf("store error must propagate: %v", err)
	}
	store.rows[recordBucket] = map[string][]byte{"io-bad": []byte(`{"schema_version":999,"id":"io-bad"}`)}
	if _, err := repository.Load(ctx, "io-bad"); err == nil {
		t.Fatal("undecodable record loaded")
	}
}

func TestRepositoryLoadDerivesClaimTokenPathFromExecutionWorkspace(t *testing.T) {
	store := newPreparationStore()
	record := repositoryRecord("io-prepare", "/repo", "199-prepare")
	store.seedRecord(t, record)
	repository := NewSQLiteRepository(store)
	before, err := repository.Load(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before.ClaimTokenPath != "" || before.ArtifactDir == "" {
		t.Fatalf("record without execution must have no claim token path: %+v", before)
	}
	commit := directRepositoryCommit(before)
	if _, err := repository.CommitDirect(context.Background(), commit); err != nil {
		t.Fatal(err)
	}
	after, err := repository.Load(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`^` + regexp.QuoteMeta(filepath.Join(commit.Workspace.Root, ".issueops", "state", "issueops-v1")) + `/[0-9a-f]{16}/lease-1\.token$`)
	if !pattern.MatchString(after.ClaimTokenPath) {
		t.Fatalf("claim token path=%q", after.ClaimTokenPath)
	}
	if string(after.RecordRaw) != string(store.mustGet(recordBucket, record.ID)) {
		t.Fatal("snapshot must carry the exact persisted bytes for CAS")
	}
}

func TestCurrentClaimTokenPathIgnoresBlankWorkspaceRoot(t *testing.T) {
	record := leasecontract.Record{ID: "io-1", Execution: &leasecontract.Execution{Workspace: leasecontract.Workspace{Root: "  "}}}
	if got := currentClaimTokenPath(record); got != "" {
		t.Fatalf("blank root produced %q", got)
	}
}

func TestEnsureRootUnclaimedUsesExecutionRootAndExcludesSelf(t *testing.T) {
	ctx := context.Background()
	if err := NewSQLiteRepository(nil).EnsureRootUnclaimed(ctx, "io-1", "/root"); err == nil || !strings.Contains(err.Error(), "store is unavailable") {
		t.Fatalf("nil store: %v", err)
	}
	store, repository, commit := preparedRepositoryFixture(t)
	if err := repository.EnsureRootUnclaimed(ctx, "io-x", "  "); err == nil || !strings.Contains(err.Error(), "canonical worktree root is required") {
		t.Fatalf("blank root: %v", err)
	}
	root := commit.Workspace.Root
	err := repository.EnsureRootUnclaimed(ctx, "io-other", root+"/")
	if err == nil || !strings.Contains(err.Error(), "io-prepared") || !strings.Contains(err.Error(), "cleanup finish --id io-prepared") {
		t.Fatalf("execution root claim must conflict with a trailing-slash spelling: %v", err)
	}
	if err := repository.EnsureRootUnclaimed(ctx, " io-prepared ", root); err != nil {
		t.Fatalf("a lifecycle must not conflict with itself: %v", err)
	}
	if err := repository.EnsureRootUnclaimed(ctx, "io-other", "/repo.worktrees/another"); err != nil {
		t.Fatalf("unrelated root rejected: %v", err)
	}
	boom := errors.New("scan failed")
	if err := ensureRootUnclaimed(inventoryErrStore{store, boom}, "io-x", root); !errors.Is(err, boom) {
		t.Fatalf("inventory error must propagate: %v", err)
	}
}

type inventoryErrStore struct {
	*preparationStore
	err error
}

func (store inventoryErrStore) GetAll(string) ([]port.RecordRow, error) { return nil, store.err }

func TestCommitDirectRejectsBeforeAnyWrite(t *testing.T) {
	ctx := context.Background()
	record := repositoryRecord("io-prepare", "/repo", "199-prepare")

	nilRepo := NewSQLiteRepository(nil)
	commit := directRepositoryCommit(preparationcontract.Snapshot{Record: record})
	if result, err := nilRepo.CommitDirect(ctx, commit); err == nil || result.ID != record.ID {
		t.Fatalf("nil store result=%+v err=%v", result, err)
	}

	store := newPreparationStore()
	store.seedRecord(t, record)
	repository := NewSQLiteRepository(store)

	missing := commit
	missing.Command.ID = "io-missing"
	if _, err := repository.CommitDirect(ctx, missing); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing record: %v", err)
	}

	other := repositoryRecord("io-other", "/repo", "199-other")
	other.WorktreePath = commit.Workspace.Root
	store.seedRecord(t, other)
	if _, err := repository.CommitDirect(ctx, commit); err == nil || !strings.Contains(err.Error(), "io-other") {
		t.Fatalf("claimed root: %v", err)
	}
	if len(store.applies) != 0 {
		t.Fatalf("rejected commit wrote %+v", store.applies)
	}
}

func TestCommitDirectRefusesAlreadyPreparedRecord(t *testing.T) {
	store := newPreparationStore()
	record := repositoryRecord("io-prepare", "/repo", "199-prepare")
	store.seedRecord(t, record)
	repository := NewSQLiteRepository(store)
	snapshot, err := repository.Load(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	commit := directRepositoryCommit(snapshot)
	if _, err := repository.CommitDirect(context.Background(), commit); err != nil {
		t.Fatal(err)
	}
	written := store.mustGet(recordBucket, record.ID)
	if _, err := repository.CommitDirect(context.Background(), commit); err == nil || !strings.Contains(err.Error(), "already prepared") {
		t.Fatalf("second commit: %v", err)
	}
	if string(store.mustGet(recordBucket, record.ID)) != string(written) || len(store.applies) != 1 {
		t.Fatal("second commit changed storage")
	}
}

func TestDirectWorkspaceAcceptsCWDThroughSymlinkAndRejectsLookalike(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(base, "repo")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(source, link); err != nil {
		t.Fatal(err)
	}
	lookalike := filepath.Join(base, "repo2")
	if err := os.Mkdir(lookalike, 0o755); err != nil {
		t.Fatal(err)
	}
	provider := &workspaceProviderFake{}
	adapter := NewDirectWorkspace(provider)
	request := preparationcontract.WorkspaceRequest{LifecycleID: "io-1", SourceRoot: source, Root: filepath.Join(base, "repo.worktrees", "x"), CWD: link}
	if _, err := adapter.Prepare(context.Background(), request); err != nil {
		t.Fatalf("symlinked source root rejected: %v", err)
	}
	request.CWD = lookalike
	if _, err := adapter.Prepare(context.Background(), request); err == nil || !strings.Contains(err.Error(), "direct prepare cwd") {
		t.Fatalf("lookalike cwd: %v", err)
	}
	request.CWD = " " + source + " "
	if _, err := adapter.ProbeAccess(context.Background(), request, "codex"); err != nil {
		t.Fatalf("padded cwd must be trimmed: %v", err)
	}
	// A blank parent resolves to the process working directory; it must still not admit that cwd.
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	request.ParentWorktree = " "
	request.CWD = workingDir
	if _, err := adapter.ProbeAccess(context.Background(), request, "codex"); err == nil {
		t.Fatal("blank parent worktree must not admit an arbitrary cwd")
	}
}

type failingPrepareFake struct{ workspaceProviderFake }

func (*failingPrepareFake) Prepare(context.Context, port.ExecutionWorkspaceRequest) (port.ExecutionWorkspaceReceipt, error) {
	return port.ExecutionWorkspaceReceipt{Root: "/partial"}, errors.New("git worktree add failed")
}

func TestDirectWorkspacePrepareSurfacesProvisionerFailure(t *testing.T) {
	adapter := NewDirectWorkspace(&failingPrepareFake{})
	_, err := adapter.Prepare(context.Background(), preparationcontract.WorkspaceRequest{SourceRoot: "/repo", Root: "/wt", CWD: "/repo"})
	if err == nil || !strings.Contains(err.Error(), "git worktree add failed") {
		t.Fatalf("err=%v", err)
	}
	if _, err := NewDirectWorkspace(nil).ProbeAccess(context.Background(), preparationcontract.WorkspaceRequest{SourceRoot: "/repo", CWD: "/repo"}, "codex"); err == nil || !strings.Contains(err.Error(), "provisioner is unavailable") {
		t.Fatalf("nil provisioner access probe: %v", err)
	}
}

func TestEnsureRootUnclaimedNormalizesRelativeRootAgainstWorkingDirectory(t *testing.T) {
	ctx := context.Background()
	_, repository, commit := preparedRepositoryFixture(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, commit.Workspace.Root)
	if err != nil || filepath.IsAbs(relative) {
		t.Fatalf("no relative spelling of %s from %s: %v", commit.Workspace.Root, cwd, err)
	}
	if err := repository.EnsureRootUnclaimed(ctx, "io-other", relative); err == nil || !strings.Contains(err.Error(), "io-prepared") {
		t.Fatalf("relative spelling of a claimed root must conflict: %v", err)
	}
}
