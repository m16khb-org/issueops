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

// noCASStore hides CompareAndApply to model a store without raw CAS support.
type noCASStore struct{ port.RecordInventoryStore }

func TestRepositoryLoadFailsClosed(t *testing.T) {
	ctx := context.Background()
	if _, err := NewSQLiteRepositoryWithDiagnosticRedactor(nil, nil).Load(ctx, "io-1"); err == nil || !strings.Contains(err.Error(), "store is unavailable") {
		t.Fatalf("nil store: %v", err)
	}
	store := newPreparationStore()
	repository := NewSQLiteRepositoryWithDiagnosticRedactor(store, nil)
	if _, err := repository.Load(ctx, "io-missing"); err == nil || !strings.Contains(err.Error(), "io-missing not found") {
		t.Fatalf("missing record: %v", err)
	}
	boom := errors.New("disk read failed")
	if _, err := NewSQLiteRepositoryWithDiagnosticRedactor(getErrStore{store, boom}, nil).Load(ctx, "io-1"); !errors.Is(err, boom) {
		t.Fatalf("store error must propagate: %v", err)
	}
	store.rows[recordBucket] = map[string][]byte{"io-bad": []byte(`{"schema_version":999,"id":"io-bad"}`)}
	if _, err := repository.Load(ctx, "io-bad"); err == nil {
		t.Fatal("undecodable record loaded")
	}
}

func TestRepositoryLoadDerivesClaimTokenPathFromExecutionWorkspace(t *testing.T) {
	store, repository, begin := newOrcaRepositoryFixture(t)
	before, err := repository.Load(context.Background(), begin.Snapshot.Record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before.ClaimTokenPath != "" || before.ArtifactDir == "" {
		t.Fatalf("record without execution must have no claim token path: %+v", before)
	}
	if _, err := repository.BeginIntent(context.Background(), begin); err != nil {
		t.Fatal(err)
	}
	after, err := repository.Load(context.Background(), begin.Snapshot.Record.ID)
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`^` + regexp.QuoteMeta(filepath.Join(begin.Workspace.Root, ".issueops", "state", "issueops-v1")) + `/[0-9a-f]{16}/lease-1\.token$`)
	if !pattern.MatchString(after.ClaimTokenPath) {
		t.Fatalf("claim token path=%q", after.ClaimTokenPath)
	}
	if string(after.RecordRaw) != string(store.mustGet(recordBucket, begin.Snapshot.Record.ID)) {
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
	if err := NewSQLiteRepositoryWithDiagnosticRedactor(nil, nil).EnsureRootUnclaimed(ctx, "io-1", "/root"); err == nil || !strings.Contains(err.Error(), "store is unavailable") {
		t.Fatalf("nil store: %v", err)
	}
	store, repository, begin := newOrcaRepositoryFixture(t)
	if err := repository.EnsureRootUnclaimed(ctx, "io-x", "  "); err == nil || !strings.Contains(err.Error(), "canonical worktree root is required") {
		t.Fatalf("blank root: %v", err)
	}
	if _, err := repository.BeginIntent(ctx, begin); err != nil {
		t.Fatal(err)
	}
	root := begin.Workspace.Root
	err := repository.EnsureRootUnclaimed(ctx, "io-other", root+"/")
	if err == nil || !strings.Contains(err.Error(), "io-orca") || !strings.Contains(err.Error(), "cleanup finish --id io-orca") {
		t.Fatalf("execution root claim must conflict with a trailing-slash spelling: %v", err)
	}
	if err := repository.EnsureRootUnclaimed(ctx, " io-orca ", root); err != nil {
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

	nilRepo := NewSQLiteRepositoryWithDiagnosticRedactor(nil, nil)
	commit := directRepositoryCommit(preparationcontract.Snapshot{Record: record})
	if result, err := nilRepo.CommitDirect(ctx, commit); err == nil || result.ID != record.ID {
		t.Fatalf("nil store result=%+v err=%v", result, err)
	}

	store := newPreparationStore()
	store.seedRecord(t, record)
	repository := NewSQLiteRepositoryWithDiagnosticRedactor(store, nil)
	bad := commit
	bad.Selection.ReadinessFingerprint = "tampered"
	if result, err := repository.CommitDirect(ctx, bad); err == nil || result.ID != record.ID || store.spans != 0 {
		t.Fatalf("tampered selection must be rejected before the span: result=%+v err=%v spans=%d", result, err, store.spans)
	}

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
	repository := NewSQLiteRepositoryWithDiagnosticRedactor(store, nil)
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

func TestBeginIntentRejectsBeforeOrWithoutPersisting(t *testing.T) {
	ctx := context.Background()
	if _, err := NewSQLiteRepositoryWithDiagnosticRedactor(nil, nil).BeginIntent(ctx, preparationapp.OrcaBegin{}); err == nil || !strings.Contains(err.Error(), "store is unavailable") {
		t.Fatalf("nil store: %v", err)
	}

	store, repository, begin := newOrcaRepositoryFixture(t)
	bad := begin
	bad.Selection.ReadinessFingerprint = "tampered"
	if _, err := repository.BeginIntent(ctx, bad); err == nil || store.spans != 0 {
		t.Fatalf("tampered selection: err=%v spans=%d", err, store.spans)
	}

	missing := begin
	missing.Snapshot.Record.ID = "io-missing"
	if _, err := repository.BeginIntent(ctx, missing); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing record: %v", err)
	}

	other := repositoryRecord("io-other", "/repo", "199-other")
	other.WorktreePath = begin.Workspace.Root
	store.seedRecord(t, other)
	if _, err := repository.BeginIntent(ctx, begin); err == nil || !strings.Contains(err.Error(), "io-other") {
		t.Fatalf("claimed root: %v", err)
	}
	delete(store.rows[recordBucket], "io-other")

	store.rows[intentBucket] = map[string][]byte{begin.OperationID: []byte("occupied")}
	before := store.mustGet(recordBucket, begin.Snapshot.Record.ID)
	if _, err := repository.BeginIntent(ctx, begin); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("occupied intent row: %v", err)
	}
	if string(store.mustGet(recordBucket, begin.Snapshot.Record.ID)) != string(before) || len(store.applies) != 0 {
		t.Fatal("record changed although the intent row insert failed")
	}
	delete(store.rows[intentBucket], begin.OperationID)

	if _, err := repository.BeginIntent(ctx, begin); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.BeginIntent(ctx, begin); err == nil || !strings.Contains(err.Error(), "IssueOps execution already exists") {
		t.Fatalf("second begin must hit the unprepared guard, not the row insert: %v", err)
	}
	if len(store.applies) != 1 {
		t.Fatalf("applies=%d", len(store.applies))
	}
}

func TestIntentWritersRequireRawCASEvidenceAndStoreSupport(t *testing.T) {
	ctx := context.Background()
	store, repository, begin := newOrcaRepositoryFixture(t)
	state, err := repository.BeginIntent(ctx, begin)
	if err != nil {
		t.Fatal(err)
	}
	noRaw := state
	noRaw.IntentRaw = nil
	if _, err := repository.MarkInvoking(ctx, noRaw); err == nil || !strings.Contains(err.Error(), "raw CAS evidence") {
		t.Fatalf("MarkInvoking: %v", err)
	}
	if err := repository.RecordFailure(ctx, noRaw, preparationcontract.InvocationUnknown, errors.New("x")); err == nil || !strings.Contains(err.Error(), "raw CAS evidence") {
		t.Fatalf("RecordFailure: %v", err)
	}
	progress, err := repository.ApplyReceipt(ctx, noRaw, repositoryOrcaReceipt(state.Intent.Stage))
	if err == nil || !progress.Pending || !strings.Contains(err.Error(), "raw CAS evidence") {
		t.Fatalf("ApplyReceipt: progress=%+v err=%v", progress, err)
	}

	unsupported := NewSQLiteRepositoryWithDiagnosticRedactor(noCASStore{store}, nil)
	state.FailureAt = "2026-08-02T00:00:01Z"
	if _, err := unsupported.MarkInvoking(ctx, state); err == nil || !strings.Contains(err.Error(), "does not support raw CAS") {
		t.Fatalf("MarkInvoking without CAS: %v", err)
	}
	if err := unsupported.RecordFailure(ctx, state, preparationcontract.InvocationUnknown, errors.New("x")); err == nil || !strings.Contains(err.Error(), "does not support raw CAS") {
		t.Fatalf("RecordFailure without CAS: %v", err)
	}
	if store.cas != 0 {
		t.Fatalf("CAS attempted %d times", store.cas)
	}
}

func TestApplyReceiptKeepsIntentPendingWhenReceiptIsRejected(t *testing.T) {
	store, repository, begin := newOrcaRepositoryFixture(t)
	state, err := repository.BeginIntent(context.Background(), begin)
	if err != nil {
		t.Fatal(err)
	}
	recordBefore, intentBefore := store.mustGet(recordBucket, begin.Snapshot.Record.ID), store.mustGet(intentBucket, begin.OperationID)
	progress, err := repository.ApplyReceipt(context.Background(), state, preparationcontract.IntentReceipt{})
	if err == nil || !progress.Pending || progress.State.Intent.Stage != preparationcontract.IntentStageWorktree {
		t.Fatalf("empty worktree receipt: progress=%+v err=%v", progress, err)
	}
	if string(store.mustGet(recordBucket, begin.Snapshot.Record.ID)) != string(recordBefore) || string(store.mustGet(intentBucket, begin.OperationID)) != string(intentBefore) || store.cas != 0 {
		t.Fatal("rejected receipt changed storage")
	}
}

func TestApplyReceiptRefusesStaleStateWithoutAdvancing(t *testing.T) {
	store, repository, begin := newOrcaRepositoryFixture(t)
	state, err := repository.BeginIntent(context.Background(), begin)
	if err != nil {
		t.Fatal(err)
	}
	state, err = repository.MarkInvoking(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	state.OwnerArtifacts = preparationcontract.OwnerArtifacts{
		PlanPath:       "/repo.worktrees/199-orca/.issueops/issues/199/artifact/plan.md",
		ClaimTokenPath: "/repo.worktrees/199-orca/.issueops/state/claim", ClaimTokenSHA256: strings.Repeat("d", 64),
		ContextPacketPath: "/repo.worktrees/199-orca/.issueops/context.json", ContextPacketSHA256: strings.Repeat("c", 64),
		OwnerPromptPath: "/repo.worktrees/199-orca/.issueops/owner.md", OwnerPromptSHA256: strings.Repeat("b", 64),
	}
	store.rows[intentBucket][begin.OperationID] = append(store.rows[intentBucket][begin.OperationID], ' ')
	progress, err := repository.ApplyReceipt(context.Background(), state, repositoryOrcaReceipt(preparationcontract.IntentStageWorktree))
	if err == nil || !strings.Contains(err.Error(), "stale raw") || !progress.Pending || progress.State.Intent.Stage != preparationcontract.IntentStageWorktree {
		t.Fatalf("progress=%+v err=%v", progress, err)
	}
}

func TestBoundedDiagnosticFallsBackWhenRedactionYieldsNothing(t *testing.T) {
	const fallback = "external operation failed"
	blank := &SQLiteRepository{redact: func(string) string { return " \n " }}
	if got := blank.boundedDiagnostic(errors.New("secret")); got != fallback {
		t.Fatalf("blank redaction=%q", got)
	}
	redacting := &SQLiteRepository{redact: func(message string) string { return "[" + message + "]" }}
	if got := redacting.boundedDiagnostic(nil); got != fallback {
		t.Fatalf("nil cause must not reach the redactor: %q", got)
	}
	if got := redacting.boundedDiagnostic(errors.New("  spaced  ")); got != "[  spaced  ]" {
		t.Fatalf("redactor output must be used as-is (trimmed by the redactor contract): %q", got)
	}
	if got := (&SQLiteRepository{}).boundedDiagnostic(errors.New("raw secret")); got != fallback {
		t.Fatalf("without a redactor the raw cause must never be persisted: %q", got)
	}
	exact := &SQLiteRepository{redact: func(string) string { return strings.Repeat("y", 4096) }}
	if got := exact.boundedDiagnostic(errors.New("x")); len(got) != 4096 {
		t.Fatalf("4096-byte message must pass untruncated: %d", len(got))
	}
}

func TestEvidenceForwardsOwnerCallbacksWithClonedInputs(t *testing.T) {
	snapshot := preparationcontract.Snapshot{RecordRaw: []byte("raw")}
	command := preparationcontract.Command{ID: "io-1"}
	adapter := NewEvidence(EvidenceDependencies{
		ReadOwner: func(_ context.Context, got preparationcontract.Snapshot, _ preparationcontract.Command) (preparationcontract.OwnerEvidence, error) {
			got.RecordRaw[0] = 'X'
			return preparationcontract.OwnerEvidence{Issue: 7}, nil
		},
		PrepareOwner: func(_ context.Context, got preparationcontract.Snapshot, _ preparationcontract.Command, intent preparationcontract.Intent, receipt preparationcontract.IntentReceipt) (preparationcontract.OwnerArtifacts, error) {
			got.RecordRaw[0] = 'Y'
			return preparationcontract.OwnerArtifacts{PlanPath: intent.Marker + "/" + receipt.RunID}, errors.New("prepare failed")
		},
	})
	owner, err := adapter.ReadOwner(context.Background(), snapshot, command)
	if err != nil || owner.Issue != 7 {
		t.Fatalf("owner=%+v err=%v", owner, err)
	}
	artifacts, err := adapter.PrepareOwner(context.Background(), snapshot, command, preparationcontract.Intent{Marker: "m"}, preparationcontract.IntentReceipt{RunID: "run"})
	if err == nil || err.Error() != "prepare failed" || artifacts.PlanPath != "m/run" {
		t.Fatalf("artifacts=%+v err=%v", artifacts, err)
	}
	if string(snapshot.RecordRaw) != "raw" {
		t.Fatalf("callbacks mutated the caller's snapshot: %q", snapshot.RecordRaw)
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

func TestRecordFailureRequiresFailureTimestampAndLeavesStorageUntouched(t *testing.T) {
	store, repository, begin := newOrcaRepositoryFixture(t)
	state, err := repository.BeginIntent(context.Background(), begin)
	if err != nil {
		t.Fatal(err)
	}
	record, intent := store.mustGet(recordBucket, begin.Snapshot.Record.ID), store.mustGet(intentBucket, begin.OperationID)
	if err := repository.RecordFailure(context.Background(), state, preparationcontract.InvocationUnknown, errors.New("x")); err == nil || !strings.Contains(err.Error(), "failure timestamp is required") {
		t.Fatalf("err=%v", err)
	}
	if string(store.mustGet(recordBucket, begin.Snapshot.Record.ID)) != string(record) || string(store.mustGet(intentBucket, begin.OperationID)) != string(intent) || store.cas != 0 {
		t.Fatal("failed RecordFailure changed storage")
	}
}
