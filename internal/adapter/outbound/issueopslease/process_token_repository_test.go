package issueopslease

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	leaseapp "issueops/internal/application/issueopslease"
	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
	"issueops/internal/port"
)

func TestInspectNativeProcessClassifiesLiveMismatchedAndDeadProcesses(t *testing.T) {
	pid := os.Getpid()
	own, err := observeReceipt(t.Context(), pid)
	if err != nil {
		t.Fatalf("observe own process: %v", err)
	}
	if own.PID != pid || own.Executable == "" {
		t.Fatalf("observed receipt=%+v", own)
	}
	if _, err := time.Parse(time.RFC3339, own.StartedAt); err != nil {
		t.Fatalf("StartedAt must be RFC3339 UTC: %v", err)
	}

	state, observed, err := InspectNativeProcess(t.Context(), own)
	if err != nil || state != "live" || observed != own {
		t.Fatalf("matching receipt state=%q observed=%+v err=%v", state, observed, err)
	}

	stale := own
	stale.StartedAt = "2000-01-01T00:00:00Z"
	state, observed, err = InspectNativeProcess(t.Context(), stale)
	if err != nil || state != "identity-mismatch" || observed != own {
		t.Fatalf("stale start state=%q observed=%+v err=%v", state, observed, err)
	}
	renamed := own
	renamed.Executable = own.Executable + "-other"
	if state, _, err := InspectNativeProcess(t.Context(), renamed); err != nil || state != "identity-mismatch" {
		t.Fatalf("different executable state=%q err=%v", state, err)
	}

	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Skipf("cannot spawn helper process: %v", err)
	}
	state, observed, err = InspectNativeProcess(t.Context(), leasedomain.ProcessReceipt{PID: cmd.Process.Pid, StartedAt: own.StartedAt, Executable: own.Executable})
	if err != nil || state != "dead" || observed != (leasedomain.ProcessReceipt{}) {
		t.Fatalf("exited process state=%q observed=%+v err=%v", state, observed, err)
	}
	if state, _, err := InspectNativeProcess(t.Context(), leasedomain.ProcessReceipt{PID: 0}); err != nil || state != "dead" {
		t.Fatalf("non-positive pid state=%q err=%v", state, err)
	}
}

func TestInspectNativeProcessReportsUnknownWhenLiveProbeIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	state, observed, err := InspectNativeProcess(ctx, leasedomain.ProcessReceipt{PID: os.Getpid()})
	if state != "unknown" || err == nil || !errors.Is(err, context.Canceled) || observed != (leasedomain.ProcessReceipt{}) {
		t.Fatalf("cancelled probe state=%q observed=%+v err=%v", state, observed, err)
	}
}

func TestObserveReceiptRejectsNonPositivePID(t *testing.T) {
	for _, pid := range []int{0, -7} {
		if _, err := observeReceipt(t.Context(), pid); err == nil || !strings.Contains(err.Error(), "must be positive") {
			t.Fatalf("pid %d err=%v", pid, err)
		}
	}
}

func TestParseProcessStartAcceptsPaddedAndUnpaddedDays(t *testing.T) {
	for _, value := range []string{"Mon Jan  5 03:04:05 2026", "  Mon Jan 5 03:04:05 2026\n"} {
		got, err := parseProcessStart(value)
		if err != nil {
			t.Fatalf("%q: %v", value, err)
		}
		if got.Year() != 2026 || got.Month() != time.January || got.Day() != 5 || got.Hour() != 3 || got.Location() != time.UTC {
			t.Fatalf("%q parsed as %v", value, got)
		}
	}
	for _, value := range []string{"", "yesterday", "2026-01-05T03:04:05Z"} {
		if _, err := parseProcessStart(value); err == nil || !strings.Contains(err.Error(), "invalid process start identity") {
			t.Fatalf("%q err=%v", value, err)
		}
	}
}

func TestReadCurrentClaimTokenRejectsUnsafeTokenFiles(t *testing.T) {
	tests := map[string]struct {
		prepare func(t *testing.T, record leasecontract.Record, path string)
		want    string
	}{
		"missing file": {
			prepare: func(t *testing.T, _ leasecontract.Record, path string) {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			},
			want: "0600 regular file",
		},
		"group-readable mode": {
			prepare: func(t *testing.T, _ leasecontract.Record, path string) {
				if err := os.Chmod(path, 0o640); err != nil {
					t.Fatal(err)
				}
			},
			want: "0600 regular file",
		},
		"oversized": {
			prepare: func(t *testing.T, _ leasecontract.Record, path string) {
				if err := os.WriteFile(path, bytes.Repeat([]byte("a"), 257), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: "oversized",
		},
		"empty after trim": {
			prepare: func(t *testing.T, _ leasecontract.Record, path string) {
				if err := os.WriteFile(path, []byte(" \n"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: "empty",
		},
		"leaf is a symlink": {
			prepare: func(t *testing.T, record leasecontract.Record, path string) {
				target := filepath.Join(record.Execution.Workspace.Root, "real.token")
				if err := os.WriteFile(target, []byte("claim-token"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			},
			want: "0600 regular file",
		},
		"leaf is a directory": {
			prepare: func(t *testing.T, _ leasecontract.Record, path string) {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			},
			want: "0600 regular file",
		},
		"ancestor is a symlink": {
			prepare: func(t *testing.T, record leasecontract.Record, path string) {
				moved := filepath.Join(record.Execution.Workspace.Root, "moved-state")
				if err := os.Rename(filepath.Dir(path), moved); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(moved, filepath.Dir(path)); err != nil {
					t.Fatal(err)
				}
			},
			want: "0600 regular file",
		},
		"ancestor is a regular file": {
			prepare: func(t *testing.T, _ leasecontract.Record, path string) {
				dir := filepath.Dir(path)
				if err := os.RemoveAll(dir); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(dir, []byte("x"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: "0600 regular file",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			record := claimableRecord(t, leasecontract.Actor{}, "claim-token")
			path := writeClaimToken(t, record, "claim-token")
			test.prepare(t, record, path)
			token, err := readCurrentClaimToken(record, path)
			if err == nil || !strings.Contains(err.Error(), test.want) || token != "" {
				t.Fatalf("token=%q err=%v want substring %q", token, err, test.want)
			}
		})
	}
}

func TestRequirePrivateRegularFileRejectsPathOutsideWorktree(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "x.token")
	if err := os.WriteFile(outside, []byte("t"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{"outside": outside, "root itself": root, "parent": filepath.Dir(root)} {
		if err := requirePrivateRegularFile(root, path, 256); err == nil || !strings.Contains(err.Error(), "inside the canonical worktree") {
			t.Fatalf("%s err=%v", name, err)
		}
	}
}

func TestReadCurrentClaimTokenTrimsAndAcceptsSizeBoundary(t *testing.T) {
	record := claimableRecord(t, leasecontract.Actor{}, "claim-token")
	path := claimTokenPath(record)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	body := strings.Repeat("a", 255) + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	token, err := readCurrentClaimToken(record, path)
	if err != nil || token != strings.Repeat("a", 255) {
		t.Fatalf("256-byte token file len=%d err=%v", len(token), err)
	}
}

type failingGetStore struct {
	claimStore
	getErr error
}

func (s *failingGetStore) Get(bucket, id string) ([]byte, bool, error) {
	if s.getErr != nil {
		return nil, false, s.getErr
	}
	return s.claimStore.Get(bucket, id)
}

func TestSQLiteRepositoryRequiresStore(t *testing.T) {
	repository := NewSQLiteRepository(nil)
	_, err := repository.Update(t.Context(), "io-x", nil, nil)
	if failure, ok := errors.AsType[*leasecontract.Failure](err); !ok || failure.Code != leasecontract.FailurePersistence {
		t.Fatalf("update err=%v", err)
	}
	_, err = repository.Claim(t.Context(), leaseapp.ClaimRepositoryRequest{})
	if failure, ok := errors.AsType[*leasecontract.Failure](err); !ok || failure.Code != leasecontract.FailurePersistence {
		t.Fatalf("claim err=%v", err)
	}
}

func activeLeaseFixture(t *testing.T) (leasecontract.Record, leasecontract.Actor, *claimStore, string) {
	t.Helper()
	actor := leasecontract.Actor{Host: "codex", SessionID: "update-session", SessionProcess: &leasecontract.ProcessReceipt{PID: 42, StartedAt: "2026-07-30T00:00:00Z", Executable: "/usr/bin/codex"}}
	record := claimableRecord(t, actor, "token")
	record.Execution.Lease = leasecontract.Lease{Generation: 3, Status: "active", Holder: &actor, ClaimedAt: "2026-07-30T00:00:01Z"}
	store := newClaimStore(t, record)
	index, err := json.Marshal(holderIndex{SchemaVersion: leasecontract.SchemaVersion, LifecycleID: record.ID, Generation: 3, Host: actor.Host, SessionID: actor.SessionID})
	if err != nil {
		t.Fatal(err)
	}
	indexKey := holderBucket + "\x00" + holderIndexKey(actor)
	store.records[indexKey] = index
	return record, actor, store, indexKey
}

func releasedLease(lease leasecontract.Lease) leasecontract.Lease {
	lease.Status = "released"
	lease.Holder = nil
	return lease
}

func TestSQLiteRepositoryUpdateAppliesTransitionAndDeletesHolderIndex(t *testing.T) {
	record, _, store, indexKey := activeLeaseFixture(t)
	result, err := NewSQLiteRepository(store).Update(t.Context(), record.ID,
		func(leaseapp.Record) error { return nil },
		func(before leaseapp.Record) (leaseapp.Record, error) {
			before.Lease = releasedLease(before.Lease)
			return before, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if result.Record.Lease.Status != "released" || result.Execution.Lease.Status != "released" {
		t.Fatalf("result=%+v", result)
	}
	if _, exists := store.records[indexKey]; exists {
		t.Fatal("holder index must be deleted together with the record update")
	}
	var stored leasecontract.Record
	if err := json.Unmarshal(store.records[recordBucket+"\x00"+record.ID], &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Execution.Lease.Status != "released" || stored.Execution.Lease.Holder != nil {
		t.Fatalf("persisted lease=%+v", stored.Execution.Lease)
	}
}

func TestSQLiteRepositoryUpdateRefusesAnotherLifecyclesHolderIndex(t *testing.T) {
	record, actor, store, indexKey := activeLeaseFixture(t)
	foreign, err := json.Marshal(holderIndex{SchemaVersion: leasecontract.SchemaVersion, LifecycleID: "io-other", Generation: 9, Host: actor.Host, SessionID: actor.SessionID})
	if err != nil {
		t.Fatal(err)
	}
	store.records[indexKey] = foreign
	before := append([]byte(nil), store.records[recordBucket+"\x00"+record.ID]...)
	transitioned := false
	_, err = NewSQLiteRepository(store).Update(t.Context(), record.ID,
		func(leaseapp.Record) error { return nil },
		func(r leaseapp.Record) (leaseapp.Record, error) { transitioned = true; return r, nil })
	if failure, ok := errors.AsType[*leasecontract.Failure](err); !ok || failure.Code != leasecontract.FailurePersistence || !strings.Contains(err.Error(), "another lifecycle") {
		t.Fatalf("err=%v", err)
	}
	if transitioned || !bytes.Equal(before, store.records[recordBucket+"\x00"+record.ID]) || !bytes.Equal(foreign, store.records[indexKey]) {
		t.Fatal("foreign index must block the update without touching state")
	}
}

func TestSQLiteRepositoryUpdateRejectsCorruptHolderIndex(t *testing.T) {
	record, _, store, indexKey := activeLeaseFixture(t)
	store.records[indexKey] = []byte("{broken")
	_, err := NewSQLiteRepository(store).Update(t.Context(), record.ID,
		func(leaseapp.Record) error { return nil },
		func(r leaseapp.Record) (leaseapp.Record, error) { return r, nil })
	if failure, ok := errors.AsType[*leasecontract.Failure](err); !ok || failure.Code != leasecontract.FailurePersistence {
		t.Fatalf("err=%v", err)
	}
}

func TestSQLiteRepositoryUpdateStopsAtValidationAndTransitionErrors(t *testing.T) {
	record, _, store, indexKey := activeLeaseFixture(t)
	recordKey := recordBucket + "\x00" + record.ID
	beforeRecord := append([]byte(nil), store.records[recordKey]...)
	repository := NewSQLiteRepository(store)

	denied := errors.New("validation denied")
	transitioned := false
	_, err := repository.Update(t.Context(), record.ID,
		func(leaseapp.Record) error { return denied },
		func(r leaseapp.Record) (leaseapp.Record, error) { transitioned = true; return r, nil })
	if !errors.Is(err, denied) || transitioned {
		t.Fatalf("validate err=%v transitioned=%v", err, transitioned)
	}

	refused := errors.New("transition refused")
	_, err = repository.Update(t.Context(), record.ID,
		func(leaseapp.Record) error { return nil },
		func(leaseapp.Record) (leaseapp.Record, error) { return leaseapp.Record{}, refused })
	if !errors.Is(err, refused) {
		t.Fatalf("transition err=%v", err)
	}
	if !bytes.Equal(beforeRecord, store.records[recordKey]) || store.records[indexKey] == nil {
		t.Fatal("rejected updates must leave record and holder index untouched")
	}
}

func TestSQLiteRepositoryUpdateReportsMissingRecordAndStoreFailures(t *testing.T) {
	record, _, store, _ := activeLeaseFixture(t)
	_, err := NewSQLiteRepository(store).Update(t.Context(), "io-absent",
		func(leaseapp.Record) error { return nil },
		func(r leaseapp.Record) (leaseapp.Record, error) { return r, nil })
	if failure, ok := errors.AsType[*leasecontract.Failure](err); !ok || failure.Code != leasecontract.FailurePersistence || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing err=%v", err)
	}

	failing := &failingGetStore{claimStore: *store, getErr: errors.New("read failed")}
	_, err = NewSQLiteRepository(failing).Update(t.Context(), record.ID,
		func(leaseapp.Record) error { return nil },
		func(r leaseapp.Record) (leaseapp.Record, error) { return r, nil })
	if failure, ok := errors.AsType[*leasecontract.Failure](err); !ok || failure.Code != leasecontract.FailurePersistence {
		t.Fatalf("get failure err=%v", err)
	}

	store.records[recordBucket+"\x00"+record.ID] = []byte("not a record")
	_, err = NewSQLiteRepository(store).Update(t.Context(), record.ID,
		func(leaseapp.Record) error { return nil },
		func(r leaseapp.Record) (leaseapp.Record, error) { return r, nil })
	if err == nil {
		t.Fatal("undecodable record must fail the update")
	}
}

type spanFailingStore struct {
	claimStore
	spanErr error
}

func (s *spanFailingStore) WithSpan(context.Context, func(context.Context) error) error {
	return s.spanErr
}

func TestSQLiteRepositoryWrapsSpanFailureAsPersistence(t *testing.T) {
	record, _, store, _ := activeLeaseFixture(t)
	failing := &spanFailingStore{claimStore: *store, spanErr: errors.New("begin failed")}
	repository := NewSQLiteRepository(failing)
	_, err := repository.Update(t.Context(), record.ID,
		func(leaseapp.Record) error { return nil },
		func(r leaseapp.Record) (leaseapp.Record, error) { return r, nil })
	if failure, ok := errors.AsType[*leasecontract.Failure](err); !ok || failure.Code != leasecontract.FailurePersistence {
		t.Fatalf("update err=%v", err)
	}
	_, err = repository.Claim(t.Context(), leaseapp.ClaimRepositoryRequest{ID: record.ID})
	if failure, ok := errors.AsType[*leasecontract.Failure](err); !ok || failure.Code != leasecontract.FailurePersistence {
		t.Fatalf("claim err=%v", err)
	}
}

func TestClaimTransactionPersistGuardsHolderIndex(t *testing.T) {
	record, actor, store, indexKey := activeLeaseFixture(t)
	recordKey := recordBucket + "\x00" + record.ID
	transaction := claimTransaction{store: store}
	beforeRecord := append([]byte(nil), store.records[recordKey]...)

	setIndex := func(index holderIndex) {
		data, err := json.Marshal(index)
		if err != nil {
			t.Fatal(err)
		}
		store.records[indexKey] = data
	}
	expectRefusal := func(name, want string) {
		t.Helper()
		_, err := transaction.Persist(t.Context(), record)
		if failure, ok := errors.AsType[*leasecontract.Failure](err); !ok || failure.Code != leasecontract.FailurePersistence || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s err=%v want %q", name, err, want)
		}
		if !bytes.Equal(beforeRecord, store.records[recordKey]) {
			t.Fatalf("%s mutated the record", name)
		}
	}

	setIndex(holderIndex{SchemaVersion: leasecontract.SchemaVersion, LifecycleID: "io-other", Generation: 3, Host: actor.Host, SessionID: actor.SessionID})
	expectRefusal("other lifecycle", "already holds active lease io-other generation 3")
	setIndex(holderIndex{SchemaVersion: leasecontract.SchemaVersion, LifecycleID: record.ID, Generation: 2, Host: actor.Host, SessionID: actor.SessionID})
	expectRefusal("other generation", "generation 2")
	setIndex(holderIndex{SchemaVersion: leasecontract.SchemaVersion + 1, LifecycleID: record.ID, Generation: 3})
	expectRefusal("future schema", "unsupported lease-holder index schema")
	store.records[indexKey] = []byte("{broken")
	expectRefusal("corrupt", "decode active lease-holder index")

	setIndex(holderIndex{SchemaVersion: leasecontract.SchemaVersion, LifecycleID: record.ID, Generation: 3, Host: actor.Host, SessionID: actor.SessionID})
	result, err := transaction.Persist(t.Context(), record)
	if err != nil || result.Execution.Lease.Generation != 3 {
		t.Fatalf("idempotent persist result=%+v err=%v", result, err)
	}
}

func TestClaimTransactionPersistWrapsApplyFailure(t *testing.T) {
	record, _, store, _ := activeLeaseFixture(t)
	store.applyErr = errors.New("write failed")
	_, err := claimTransaction{store: store}.Persist(t.Context(), record)
	if failure, ok := errors.AsType[*leasecontract.Failure](err); !ok || failure.Code != leasecontract.FailurePersistence {
		t.Fatalf("err=%v", err)
	}
}

func TestDecodeMutableLeaseRecordClassifiesFailures(t *testing.T) {
	record, _, store, _ := activeLeaseFixture(t)
	good := store.records[recordBucket+"\x00"+record.ID]
	if _, err := decodeMutableLeaseRecord(record.ID, good); err != nil {
		t.Fatalf("valid record: %v", err)
	}
	_, err := decodeMutableLeaseRecord(record.ID, []byte("garbage"))
	if _, ok := errors.AsType[*leasecontract.Failure](err); !ok {
		t.Fatalf("garbage err=%v", err)
	}
	if _, err := decodeMutableLeaseRecord("io-different-id", good); err == nil {
		t.Fatal("record id mismatch must fail")
	}
}

func TestUTCClockReturnsCurrentUTCTime(t *testing.T) {
	before := time.Now()
	got := UTCClock{}.Now()
	after := time.Now()
	if got.Location() != time.UTC || got.Before(before.Add(-time.Second)) || got.After(after.Add(time.Second)) {
		t.Fatalf("now=%v outside [%v,%v] or not UTC", got, before, after)
	}
}

var _ port.TransactionalRecordStore = (*failingGetStore)(nil)
