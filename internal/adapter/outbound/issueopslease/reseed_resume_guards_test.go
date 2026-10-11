package issueopslease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	leaseapp "issueops/internal/application/issueopslease"
	model "issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
	"issueops/internal/port"
)

func TestResumeArtifactsFailsClosedWithoutReaderAndPropagatesReaderError(t *testing.T) {
	var nilAdapter *ResumeArtifacts
	for name, adapter := range map[string]*ResumeArtifacts{"nil": nilAdapter, "empty": NewResumeArtifacts(nil)} {
		if _, err := adapter.ReadAndVerify(t.Context(), leasecontract.Record{}); err == nil || !strings.Contains(err.Error(), "reader is required") {
			t.Fatalf("%s err=%v", name, err)
		}
	}
	wantErr := errors.New("digest drift")
	_, err := NewResumeArtifacts(func(context.Context, leasecontract.Record) (leasecontract.ResumeArtifacts, error) {
		return leasecontract.ResumeArtifacts{ContextPacketSHA256: "partial"}, wantErr
	}).ReadAndVerify(t.Context(), leasecontract.Record{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err=%v", err)
	}
}

func TestSQLiteReseedFenceValidatesOpenerAndScopesStoreByLifecycle(t *testing.T) {
	if _, err := NewSQLiteReseedFence(t.TempDir(), nil); err == nil || !strings.Contains(err.Error(), "opener is required") {
		t.Fatalf("nil opener err=%v", err)
	}

	stateRoot := t.TempDir()
	var opened []string
	openErr := errors.New("cannot open fence store")
	fail := false
	fence, err := NewSQLiteReseedFence(stateRoot, func(root string) (port.TransactionalRecordStore, error) {
		opened = append(opened, root)
		if fail {
			return nil, openErr
		}
		return sqlstore.Open(root)
	})
	if err != nil {
		t.Fatal(err)
	}
	ran := false
	if err := fence.Within(t.Context(), "io-one", func(context.Context) error { ran = true; return nil }); err != nil || !ran {
		t.Fatalf("within ran=%v err=%v", ran, err)
	}
	sum := sha256.Sum256([]byte("io-one"))
	want := filepath.Join(stateRoot, "issueops-reseed-fence", hex.EncodeToString(sum[:]))
	if len(opened) != 1 || opened[0] != want {
		t.Fatalf("opened=%v want [%s]", opened, want)
	}

	bodyErr := errors.New("body failed")
	if err := fence.Within(t.Context(), "io-one", func(context.Context) error { return bodyErr }); !errors.Is(err, bodyErr) {
		t.Fatalf("body error must propagate: %v", err)
	}

	fail, ran = true, false
	if err := fence.Within(t.Context(), "io-two", func(context.Context) error { ran = true; return nil }); !errors.Is(err, openErr) || ran {
		t.Fatalf("opener failure ran=%v err=%v", ran, err)
	}
}

func seededReseedStore(t *testing.T) (*sqlstore.DB, leasecontract.Record) {
	t.Helper()
	db, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	record := reseedRepositoryRecord()
	data, err := leasecontract.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Apply(t.Context(), []port.RecordMutation{{Bucket: recordBucket, ID: record.ID, Data: data}}); err != nil {
		t.Fatal(err)
	}
	return db, record
}

func TestReseedRepositoryRequiresStore(t *testing.T) {
	var nilRepository *ReseedRepository
	for name, repository := range map[string]*ReseedRepository{"nil repository": nilRepository, "nil store": NewReseedRepository(nil)} {
		_, err := repository.LoadSnapshot(t.Context(), "io-x")
		if failure, ok := errors.AsType[*leasecontract.Failure](err); !ok || failure.Code != leasecontract.FailurePersistence {
			t.Fatalf("%s load err=%v", name, err)
		}
		_, err = repository.CommitReseed(t.Context(), leaseapp.ReseedSnapshot{}, leaseapp.Record{})
		if failure, ok := errors.AsType[*leasecontract.Failure](err); !ok || failure.Code != leasecontract.FailurePersistence {
			t.Fatalf("%s commit err=%v", name, err)
		}
	}
}

func TestReseedRepositoryLoadSnapshotReportsMissingAndUndecodableRecords(t *testing.T) {
	db, record := seededReseedStore(t)
	repository := NewReseedRepository(db)
	if _, err := repository.LoadSnapshot(t.Context(), "io-absent"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing err=%v", err)
	}
	if err := db.Apply(t.Context(), []port.RecordMutation{{Bucket: recordBucket, ID: record.ID, Data: []byte("garbage")}}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.LoadSnapshot(t.Context(), record.ID); err == nil {
		t.Fatal("undecodable record must fail")
	}
}

func TestReseedRepositoryCommitRejectsMissingRecordCleanupAttemptAndGenerationDrift(t *testing.T) {
	db, record := seededReseedStore(t)
	repository := NewReseedRepository(db)
	snapshot, err := repository.LoadSnapshot(t.Context(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	recordKeyRaw := func() []byte {
		data, _, getErr := db.Get(recordBucket, record.ID)
		if getErr != nil {
			t.Fatal(getErr)
		}
		return data
	}
	before := recordKeyRaw()

	fenced := snapshot.Record
	stable := fenced.Stable
	stable.CleanupAttempt = &model.IssueOpsCleanupAttempt{Operation: "finish", Token: strings.Repeat("a", 64), StartedAt: "2026-09-29T00:00:00Z"}
	fenced.Stable = stable
	if _, err := repository.CommitReseed(t.Context(), snapshot, fenced); err == nil || !strings.Contains(err.Error(), "cleanup finish") {
		t.Fatalf("cleanup attempt err=%v", err)
	}

	drifted := snapshot
	drifted.Record.Lease.Generation = snapshot.Record.Lease.Generation + 1
	if _, err := repository.CommitReseed(t.Context(), drifted, snapshot.Record); err == nil || !strings.Contains(err.Error(), "stale lease generation") {
		t.Fatalf("generation drift err=%v", err)
	}

	missing := snapshot
	missing.Record.ID = "io-absent"
	if _, err := repository.CommitReseed(t.Context(), missing, snapshot.Record); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing record err=%v", err)
	}
	if string(recordKeyRaw()) != string(before) {
		t.Fatal("rejected commits must not change the stored record")
	}
}

func TestReseedRepositoryCommitWrapsStoreFailures(t *testing.T) {
	record := reseedRepositoryRecord()
	data, err := leasecontract.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	store := &claimStore{records: map[string][]byte{recordBucket + "\x00" + record.ID: data}}
	repository := NewReseedRepository(store)
	snapshot, err := repository.LoadSnapshot(t.Context(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	store.applyErr = errors.New("disk full")
	_, err = repository.CommitReseed(t.Context(), snapshot, snapshot.Record)
	if failure, ok := errors.AsType[*leasecontract.Failure](err); !ok || failure.Code != leasecontract.FailurePersistence {
		t.Fatalf("apply failure err=%v", err)
	}

	span := &spanFailingStore{claimStore: *store, spanErr: errors.New("begin failed")}
	span.applyErr = nil
	_, err = NewReseedRepository(span).CommitReseed(t.Context(), snapshot, snapshot.Record)
	if failure, ok := errors.AsType[*leasecontract.Failure](err); !ok || failure.Code != leasecontract.FailurePersistence {
		t.Fatalf("span failure err=%v", err)
	}
}

func TestReconcileStageExecutorDelegatesAndPassesRoleAgentArgs(t *testing.T) {
	wantReceipt := leasecontract.ReconcileStageReceipt{TerminalPTYID: "pty"}
	var gotArgs []string
	executor := NewReconcileStageExecutor(
		func(context.Context, leaseapp.ReconcileIntentState) (leasecontract.ReconcileStageInventory, bool, error) {
			return leasecontract.ReconcileStageInventory{AuthoritativeZero: true}, true, nil
		},
		func(_ context.Context, intent leaseapp.ReconcileIntentState, args []string) (leasecontract.ReconcileStageReceipt, string, error) {
			gotArgs = args
			return wantReceipt, "invoked:" + intent.OperationID, nil
		},
	)
	inventory, ok, err := executor.Inspect(t.Context(), leaseapp.ReconcileIntentState{})
	if err != nil || !ok || !inventory.AuthoritativeZero {
		t.Fatalf("inspect=%+v ok=%v err=%v", inventory, ok, err)
	}
	receipt, state, err := executor.Invoke(t.Context(), leaseapp.ReconcileIntentState{OperationID: "op"}, []string{"--role", "owner"})
	if err != nil || receipt != wantReceipt || state != "invoked:op" || len(gotArgs) != 2 || gotArgs[1] != "owner" {
		t.Fatalf("receipt=%+v state=%q args=%v err=%v", receipt, state, gotArgs, err)
	}
}
