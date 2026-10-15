package issueopscompletion

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	completioncontract "issueops/internal/contract/issueopscompletion"
	leasecontract "issueops/internal/contract/issueopslease"
	"issueops/internal/port"
)

// faultStore는 memoryStore에 Get/Apply/WithSpan 오류를 주입한다.
type faultStore struct {
	*memoryStore
	getErr   map[string]error
	applyErr error
	batches  [][]port.RecordMutation
	spanErr  error
}

func (s *faultStore) Get(bucket, id string) ([]byte, bool, error) {
	if err := s.getErr[bucket]; err != nil {
		return nil, false, err
	}
	return s.memoryStore.Get(bucket, id)
}

func (s *faultStore) Apply(ctx context.Context, m []port.RecordMutation) error {
	s.batches = append(s.batches, append([]port.RecordMutation(nil), m...))
	if s.applyErr != nil {
		return s.applyErr
	}
	return s.memoryStore.Apply(ctx, m)
}

func (s *faultStore) WithSpan(ctx context.Context, fn func(context.Context) error) error {
	if err := fn(ctx); err != nil {
		return err
	}
	return s.spanErr
}

func seededStore(t *testing.T, record leasecontract.Record) *faultStore {
	t.Helper()
	raw, err := leasecontract.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	return &faultStore{memoryStore: newMemoryStore(map[string][]byte{recordKey(recordBucket, record.ID): raw})}
}

func changing(before completioncontract.RecordSnapshot) (completioncontract.RecordSnapshot, bool, error) {
	before.Phase = "done"
	return before, true, nil
}

func TestRepositoryUpdateRequiresStore(t *testing.T) {
	for name, repo := range map[string]*Repository{"nil repository": nil, "nil store": NewRepository(nil)} {
		if _, err := repo.Update(t.Context(), "io-1", changing); err == nil || !strings.Contains(err.Error(), "transactional record store is required") {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
}

func TestRepositoryUpdateRejectsMissingRecord(t *testing.T) {
	store := &faultStore{memoryStore: newMemoryStore(nil)}
	_, err := NewRepository(store).Update(t.Context(), "io-404", changing)
	if err == nil || !strings.Contains(err.Error(), "issueops record io-404 not found") {
		t.Fatalf("err = %v", err)
	}
}

func TestRepositoryUpdatePropagatesRecordReadError(t *testing.T) {
	record, _ := completionRepositoryRecord(t)
	store := seededStore(t, record)
	boom := errors.New("read boom")
	store.getErr = map[string]error{recordBucket: boom}
	if _, err := NewRepository(store).Update(t.Context(), record.ID, changing); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}

func TestRepositoryUpdateRejectsUndecodableRecord(t *testing.T) {
	store := &faultStore{memoryStore: newMemoryStore(map[string][]byte{recordKey(recordBucket, "io-1"): []byte("{not json")})}
	called := false
	_, err := NewRepository(store).Update(t.Context(), "io-1", func(s completioncontract.RecordSnapshot) (completioncontract.RecordSnapshot, bool, error) {
		called = true
		return s, false, nil
	})
	if err == nil || called {
		t.Fatalf("called=%v err=%v", called, err)
	}
}

func TestRepositoryUpdatePropagatesTransitionError(t *testing.T) {
	record, actor := completionRepositoryRecord(t)
	store := seededStore(t, record)
	boom := errors.New("transition boom")
	_, err := NewRepository(store).Update(t.Context(), record.ID, func(s completioncontract.RecordSnapshot) (completioncontract.RecordSnapshot, bool, error) {
		return s, true, boom
	})
	if !errors.Is(err, boom) || store.applyCalls != 0 {
		t.Fatalf("err=%v applyCalls=%d", err, store.applyCalls)
	}
	_ = actor
}

func TestRepositoryUnchangedReturnsSnapshotAndExecution(t *testing.T) {
	record, _ := completionRepositoryRecord(t)
	store := seededStore(t, record)
	result, err := NewRepository(store).Update(t.Context(), record.ID, func(s completioncontract.RecordSnapshot) (completioncontract.RecordSnapshot, bool, error) {
		return s, false, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Record.Prepared || result.Execution.Lease.Status != "active" {
		t.Fatalf("execution = %+v", result.Execution)
	}
	snap := result.Record
	if snap.CanonicalRoot != "/worktree" || snap.BaseBranch != "main" || snap.Phase != "pr" {
		t.Fatalf("snapshot = %+v", snap)
	}
	if snap.Lease.Holder == nil || snap.Lease.Holder.SessionID != "session-198" || snap.Lease.Holder.Process == nil || snap.Lease.Holder.Process.PID != 198 || snap.Lease.Holder.Process.Executable != "/bin/codex" {
		t.Fatalf("lease holder = %+v", snap.Lease.Holder)
	}
	if snap.Artifact == nil || snap.Artifact.URL != "https://github.com/acme/repo/pull/198" || snap.Artifact.TargetBranch != "main" || len(snap.Artifact.Labels) != 1 || snap.Artifact.Assignees[0] != "m16khb" {
		t.Fatalf("artifact = %+v", snap.Artifact)
	}
	if entry, ok := snap.Ledger["pr"]; !ok || entry.EnteredAt != "2026-08-02T00:00:03Z" {
		t.Fatalf("ledger = %+v", snap.Ledger)
	}
	if snap.Completion != nil {
		t.Fatalf("unexpected completion %+v", snap.Completion)
	}
}

func TestRepositoryUnchangedWithoutExecutionReportsUnprepared(t *testing.T) {
	record, _ := completionRepositoryRecord(t)
	record.Execution = nil
	record.PhaseLedger = nil
	store := seededStore(t, record)
	result, err := NewRepository(store).Update(t.Context(), record.ID, func(s completioncontract.RecordSnapshot) (completioncontract.RecordSnapshot, bool, error) {
		return s, false, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Record.Prepared || result.Record.Lease.Holder != nil || result.Record.Ledger == nil || len(result.Record.Ledger) != 0 {
		t.Fatalf("record = %+v", result.Record)
	}
}

func TestRepositoryChangeWithoutExecutionIsNotPrepared(t *testing.T) {
	record, _ := completionRepositoryRecord(t)
	record.Execution = nil
	store := seededStore(t, record)
	_, err := NewRepository(store).Update(t.Context(), record.ID, changing)
	if !errors.Is(err, leasecontract.ErrExecutionNotPrepared) || store.applyCalls != 0 {
		t.Fatalf("err=%v applyCalls=%d", err, store.applyCalls)
	}
}

func TestRepositoryChangeRejectsLeaseWithoutHolder(t *testing.T) {
	record, _ := completionRepositoryRecord(t)
	record.Execution.Lease.Holder = nil
	record.Execution.Lease.Status = "released"
	record.Execution.Lease.ReleasedAt = "2026-08-02T00:30:00Z"
	store := seededStore(t, record)
	_, err := NewRepository(store).Update(t.Context(), record.ID, changing)
	if err == nil || !strings.Contains(err.Error(), "missing its holder") || store.applyCalls != 0 {
		t.Fatalf("err=%v applyCalls=%d", err, store.applyCalls)
	}
}

func TestRepositoryChangeRefusesForeignHolderIndex(t *testing.T) {
	record, actor := completionRepositoryRecord(t)
	store := seededStore(t, record)
	foreign, _ := json.Marshal(holderIndex{SchemaVersion: 1, LifecycleID: "io-other", Generation: 1, Host: actor.Host, SessionID: actor.SessionID})
	store.records[recordKey(holderBucket, holderIndexKey(actor))] = foreign
	_, err := NewRepository(store).Update(t.Context(), record.ID, changing)
	if err == nil || !strings.Contains(err.Error(), "another lifecycle's lease-holder index") {
		t.Fatalf("err = %v", err)
	}
	if store.applyCalls != 0 {
		t.Fatal("foreign index must block the whole write")
	}
	if _, ok := store.records[recordKey(holderBucket, holderIndexKey(actor))]; !ok {
		t.Fatal("foreign index was deleted")
	}
}

func TestRepositoryChangeRejectsCorruptHolderIndex(t *testing.T) {
	record, actor := completionRepositoryRecord(t)
	store := seededStore(t, record)
	store.records[recordKey(holderBucket, holderIndexKey(actor))] = []byte("{oops")
	_, err := NewRepository(store).Update(t.Context(), record.ID, changing)
	if err == nil || !strings.Contains(err.Error(), "decode active lease-holder index") || store.applyCalls != 0 {
		t.Fatalf("err=%v applyCalls=%d", err, store.applyCalls)
	}
}

func TestRepositoryChangePropagatesHolderIndexReadError(t *testing.T) {
	record, _ := completionRepositoryRecord(t)
	store := seededStore(t, record)
	boom := errors.New("index boom")
	store.getErr = map[string]error{holderBucket: boom}
	if _, err := NewRepository(store).Update(t.Context(), record.ID, changing); !errors.Is(err, boom) || store.applyCalls != 0 {
		t.Fatalf("err=%v applyCalls=%d", err, store.applyCalls)
	}
}

func TestRepositoryChangeWithoutHolderIndexWritesOnlyRecord(t *testing.T) {
	record, _ := completionRepositoryRecord(t)
	store := seededStore(t, record)
	if _, err := NewRepository(store).Update(t.Context(), record.ID, changing); err != nil {
		t.Fatal(err)
	}
	persisted, err := leasecontract.Decode(record.ID, store.records[recordKey(recordBucket, record.ID)])
	if err != nil || persisted.Phase != "done" {
		t.Fatalf("persisted phase=%q err=%v", persisted.Phase, err)
	}
}

func TestRepositoryChangePropagatesApplyError(t *testing.T) {
	record, _ := completionRepositoryRecord(t)
	store := seededStore(t, record)
	boom := errors.New("apply boom")
	store.applyErr = boom
	if _, err := NewRepository(store).Update(t.Context(), record.ID, changing); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}

func TestRepositoryChangeAppliesRecordAndIndexDeleteInOneBatch(t *testing.T) {
	record, actor := completionRepositoryRecord(t)
	store := seededStore(t, record)
	idx, _ := json.Marshal(holderIndex{SchemaVersion: 1, LifecycleID: record.ID, Host: actor.Host, SessionID: actor.SessionID})
	store.records[recordKey(holderBucket, holderIndexKey(actor))] = idx
	if _, err := NewRepository(store).Update(t.Context(), record.ID, changing); err != nil {
		t.Fatal(err)
	}
	if len(store.batches) != 1 || len(store.batches[0]) != 2 {
		t.Fatalf("batches = %+v", store.batches)
	}
	batch := store.batches[0]
	if batch[0].Bucket != recordBucket || batch[0].ID != record.ID || batch[0].Delete {
		t.Fatalf("record mutation = %+v", batch[0])
	}
	if batch[1].Bucket != holderBucket || batch[1].ID != holderIndexKey(actor) || !batch[1].Delete {
		t.Fatalf("index mutation = %+v", batch[1])
	}
}

func TestRepositoryUpdateSurfacesSpanError(t *testing.T) {
	record, _ := completionRepositoryRecord(t)
	store := seededStore(t, record)
	boom := errors.New("span commit boom")
	store.spanErr = boom
	result, err := NewRepository(store).Update(t.Context(), record.ID, changing)
	if !errors.Is(err, boom) || result.Record.ID != "" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestSnapshotRejectsMalformedSidecars(t *testing.T) {
	cases := map[string]func(*leasecontract.Record){
		"branch prepare":  func(r *leasecontract.Record) { r.BranchPrepare = json.RawMessage(`{"base_branch":1}`) },
		"remote artifact": func(r *leasecontract.Record) { r.RemoteArtifact = json.RawMessage(`{"labels":"x"}`) },
		"phase ledger":    func(r *leasecontract.Record) { r.PhaseLedger = json.RawMessage(`{"pr":"x"}`) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			record, _ := completionRepositoryRecord(t)
			mutate(&record)
			if _, err := snapshot(record); err == nil {
				t.Fatal("malformed sidecar accepted")
			}
		})
	}
}

func TestRepositoryClearingCompletionPersistsNil(t *testing.T) {
	record, _ := completionRepositoryRecord(t)
	record.Execution.Completion = &leasecontract.Completion{Generation: 1, FinalHead: strings.Repeat("a", 40), VerificationReportPath: "/worktree/r.json", Verification: []string{"v"}, RemoteArtifactURL: "https://github.com/acme/repo/pull/198", CompletedAt: "2026-08-02T00:00:00Z"}
	store := seededStore(t, record)
	var seen *completioncontract.Completion
	_, err := NewRepository(store).Update(t.Context(), record.ID, func(s completioncontract.RecordSnapshot) (completioncontract.RecordSnapshot, bool, error) {
		seen = s.Completion
		s.Completion = nil
		return s, true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen == nil || seen.FinalHead != strings.Repeat("a", 40) || seen.Verification[0] != "v" {
		t.Fatalf("snapshot completion = %+v", seen)
	}
	persisted, err := leasecontract.Decode(record.ID, store.records[recordKey(recordBucket, record.ID)])
	if err != nil || persisted.Execution.Completion != nil {
		t.Fatalf("completion = %+v err=%v", persisted.Execution.Completion, err)
	}
}

func TestUTCClockReturnsUTCCurrentTime(t *testing.T) {
	before := time.Now()
	got := UTCClock{}.Now()
	if got.Location() != time.UTC || got.Before(before.Add(-time.Second)) || got.After(time.Now().Add(time.Second)) {
		t.Fatalf("Now() = %v", got)
	}
}

func TestEnvironmentPathsMatchFollowsSymlinks(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	if !NewEnvironment().PathsMatch(link, dir) {
		t.Fatal("symlink and target must match")
	}
	missing := filepath.Join(dir, "missing")
	if !NewEnvironment().PathsMatch(missing+string(filepath.Separator), missing) {
		t.Fatal("unresolvable path must still compare cleaned")
	}
	if NewEnvironment().PathsMatch(missing, filepath.Join(dir, "other")) {
		t.Fatal("distinct unresolvable paths must not match")
	}
}

func TestEnvironmentVerifyReport(t *testing.T) {
	env := NewEnvironment()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(root, "report.json")
	if err := os.WriteFile(report, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := env.VerifyReport(root, "  "+report+"  ")
	if err != nil || got != report {
		t.Fatalf("got=%q err=%v", got, err)
	}

	if _, err := env.VerifyReport(root, filepath.Join(root, "absent.json")); err == nil || !strings.Contains(err.Error(), "must exist") {
		t.Fatalf("missing: %v", err)
	}

	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := env.VerifyReport(root, outside); err == nil || !strings.Contains(err.Error(), "inside the canonical worktree") {
		t.Fatalf("outside: %v", err)
	}

	escape := filepath.Join(root, "escape.json")
	if err := os.Symlink(outside, escape); err != nil {
		t.Fatal(err)
	}
	if _, err := env.VerifyReport(root, escape); err == nil || !strings.Contains(err.Error(), "inside the canonical worktree") {
		t.Fatalf("symlink escape: %v", err)
	}

	inner := filepath.Join(root, "dir")
	if err := os.Mkdir(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := env.VerifyReport(root, inner); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("directory: %v", err)
	}
}
