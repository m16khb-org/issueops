package issueopslease

import (
	"context"
	"errors"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	leaseapp "issueops/internal/application/issueopslease"
	leasecontract "issueops/internal/contract/issueopslease"
	"issueops/internal/port"
)

// bucketGetFailStore는 지정한 bucket의 Get만 실패시키고, CompareAndApply는 주어진 오류로 대체할 수 있다.
type bucketGetFailStore struct {
	*sqlstore.DB
	failBucket string
	getErr     error
	casErr     error
}

func (s *bucketGetFailStore) Get(bucket, id string) ([]byte, bool, error) {
	if bucket == s.failBucket {
		return nil, false, s.getErr
	}
	return s.DB.Get(bucket, id)
}

func (s *bucketGetFailStore) CompareAndApply(ctx context.Context, expected []port.ExpectedRecord, mutations []port.RecordMutation) error {
	if s.casErr != nil {
		return s.casErr
	}
	return s.DB.CompareAndApply(ctx, expected, mutations)
}

type rawCASFailureStub struct{ bucket string }

func (e rawCASFailureStub) Error() string        { return "raw CAS mismatch" }
func (e rawCASFailureStub) FailedBucket() string { return e.bucket }

func persistenceFailure(t *testing.T, err error) *leasecontract.Failure {
	t.Helper()
	failure, ok := errors.AsType[*leasecontract.Failure](err)
	if !ok || failure.Code != leasecontract.FailurePersistence {
		t.Fatalf("want persistence failure, got %v", err)
	}
	return failure
}

func TestResumeRepositoryLoadSnapshotWrapsStoreReadFailureAsPersistence(t *testing.T) {
	ctx := context.Background()
	_, store := newResumeRepositoryStore(t, resumeRepositoryRecord(t, 4))

	boom := errors.New("disk gone")
	failing := NewResumeRepository(&bucketGetFailStore{DB: store, failBucket: recordBucket, getErr: boom})
	if _, err := failing.LoadSnapshot(ctx, "io-resume-repository", 4); !errors.Is(err, boom) {
		t.Fatalf("store read err=%v", err)
	} else {
		persistenceFailure(t, err)
	}
}

func TestResumeRepositoryLoadSnapshotRejectsRecordWithoutExecution(t *testing.T) {
	record := resumeRepositoryRecord(t, 4)
	record.Execution = nil
	_, store := newResumeRepositoryStore(t, record)
	_, err := NewResumeRepository(store).LoadSnapshot(context.Background(), record.ID, 4)
	if !errors.Is(err, leasecontract.ErrExecutionNotPrepared) {
		t.Fatalf("err=%v", err)
	}
	persistenceFailure(t, err)
}

func TestResumeRepositoryLoadIntentPropagatesReadErrorsAndRejectsCorruptPayload(t *testing.T) {
	ctx := context.Background()
	repository, state, store := seededResumeIntent(t)

	boom := errors.New("bucket unreadable")
	for _, bucket := range []string{recordBucket, "external_intent_v1"} {
		failing := NewResumeRepository(&bucketGetFailStore{DB: store, failBucket: bucket, getErr: boom})
		if _, err := failing.LoadIntent(ctx, state.Progress); !errors.Is(err, boom) {
			t.Fatalf("%s read err=%v", bucket, err)
		}
	}

	if err := store.Apply(ctx, []port.RecordMutation{{Bucket: "external_intent_v1", ID: state.OperationID, Data: []byte("{not json")}}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.LoadIntent(ctx, state.Progress); err == nil || strings.Contains(err.Error(), "payload is missing") {
		t.Fatalf("corrupt intent must fail decoding, err=%v", err)
	}
}

func TestResumeRepositoryApplyReceiptTranslatesRawCASFailureByBucket(t *testing.T) {
	ctx := context.Background()
	_, state, store := seededResumeIntent(t)
	for bucket, want := range map[string]string{recordBucket: "stale raw record snapshot", "external_intent_v1": "stale raw intent snapshot"} {
		repo := NewResumeRepository(&bucketGetFailStore{DB: store, casErr: rawCASFailureStub{bucket: bucket}})
		if _, err := repo.ApplyReceipt(ctx, state, leasecontract.ResumeStageReceipt{TerminalPTYID: "pty-new"}); err == nil || err.Error() != want {
			t.Fatalf("bucket %s err=%v want %q", bucket, err, want)
		}
	}
	boom := errors.New("apply exploded")
	repo := NewResumeRepository(&bucketGetFailStore{DB: store, casErr: boom})
	if _, err := repo.ApplyReceipt(ctx, state, leasecontract.ResumeStageReceipt{TerminalPTYID: "pty-new"}); !errors.Is(err, boom) {
		t.Fatalf("non-CAS err=%v", err)
	}
}

func TestResumeRepositoryApplyReceiptRejectsCorruptIntentPayload(t *testing.T) {
	ctx := context.Background()
	_, state, _ := seededResumeIntent(t)
	receipt := leasecontract.ResumeStageReceipt{TerminalPTYID: "pty-new"}

	corrupt := state
	corrupt.IntentRaw = []byte("{not json")
	_, _, store := seededResumeIntent(t)
	if _, err := NewResumeRepository(store).ApplyReceipt(ctx, corrupt, receipt); err == nil {
		t.Fatal("corrupt intent payload was accepted")
	}
}

func TestResumeRepositoryBeginIntentTranslatesRawCASFailureByBucket(t *testing.T) {
	ctx := context.Background()
	record := resumeRepositoryRecord(t, 4)
	repo := t.TempDir() + "/repo"
	record.Repo, record.Branch = repo, "193-resume"
	record.BranchPrepare = []byte(`{"provider":"github","issue_url":"https://github.com/m16khb/issueops/issues/193","link_verified":true,"base_branch":"main","base_sha":"base"}`)
	record.Execution.Workspace.SourceRoot = repo
	record.Execution.Workspace.Root = repo + ".worktrees/" + record.Branch
	record.Execution.Workspace.Branch = record.Branch
	record.Execution.Workspace.BaseHead = "base"
	_, store := newResumeRepositoryStore(t, record)
	snapshot, err := NewResumeRepository(store).LoadSnapshot(ctx, record.ID, 4)
	if err != nil {
		t.Fatal(err)
	}
	root := record.Execution.Workspace.Root
	artifacts := leasecontract.ResumeArtifacts{IssueBodySHA256: strings.Repeat("a", 64), OwnerPromptPath: root + "/prompt", OwnerPromptSHA256: strings.Repeat("c", 64), ContextPacketPath: root + "/packet", ContextPacketSHA256: strings.Repeat("d", 64)}
	for bucket, want := range map[string]string{recordBucket: "stale raw record snapshot", "external_intent_v1": "stale raw intent snapshot"} {
		repository := NewResumeRepository(&bucketGetFailStore{DB: store, casErr: rawCASFailureStub{bucket: bucket}})
		if _, err := repository.BeginIntent(ctx, snapshot, artifacts, resumeRepositoryPlan(), strings.Repeat("e", 32)); err == nil || err.Error() != want {
			t.Fatalf("bucket %s err=%v want %q", bucket, err, want)
		}
	}
	boom := errors.New("apply exploded")
	if _, err := NewResumeRepository(&bucketGetFailStore{DB: store, casErr: boom}).BeginIntent(ctx, snapshot, artifacts, resumeRepositoryPlan(), strings.Repeat("e", 32)); !errors.Is(err, boom) {
		t.Fatalf("non-CAS err=%v", err)
	}
}

func TestReconcileRepositoryApplyReceiptRejectsCorruptIntentAndMissingExecution(t *testing.T) {
	ctx := context.Background()
	_, sealed, store := seededResumeIntent(t)
	intent := leaseapp.ReconcileIntentState{
		Progress:    leaseapp.ReconcileProgress{Record: sealed.Progress.Record.Stable, Pending: true, NextStage: sealed.Stage},
		OperationID: sealed.OperationID, Stage: sealed.Stage, InvocationState: sealed.InvocationState,
		RecordRaw: sealed.RecordRaw, IntentRaw: sealed.IntentRaw,
	}
	receipt := leasecontract.ReconcileStageReceipt{TerminalPTYID: "pty-recovered"}

	if _, err := NewReconcileRepositoryWithSnapshot(nil, nil, nil, nil, nil).ApplyReceipt(ctx, intent, receipt); err == nil || !strings.Contains(err.Error(), "reconcile record store is required") {
		t.Fatalf("nil store err=%v", err)
	}
	corrupt := intent
	corrupt.IntentRaw = []byte("{not json")
	if _, err := NewReconcileRepositoryWithSnapshot(store, nil, nil, nil, nil).ApplyReceipt(ctx, corrupt, receipt); err == nil {
		t.Fatal("corrupt intent payload was accepted")
	}
	noExecution := intent
	noExecution.Progress.Record.Execution = nil
	if _, err := NewReconcileRepositoryWithSnapshot(store, nil, nil, nil, nil).ApplyReceipt(ctx, noExecution, receipt); err == nil || !strings.Contains(err.Error(), "reconcile execution is required") {
		t.Fatalf("record without execution err=%v", err)
	}
}
