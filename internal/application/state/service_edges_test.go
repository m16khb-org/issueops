package state

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"strconv"
	"testing"
	"time"

	statecontract "issueops/internal/contract/state"
	stateport "issueops/internal/port/state"
)

// faultStore wraps memoryStore and injects failures at individual port calls.
type faultStore struct {
	*memoryStore
	mutateErr error
	listErr   error
}

func (s *faultStore) Mutate(ctx context.Context, mutations []stateport.Mutation) error {
	if s.mutateErr != nil {
		return s.mutateErr
	}
	return s.memoryStore.Mutate(ctx, mutations)
}

func (s *faultStore) List(bucket string) ([]string, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	keys, err := s.memoryStore.List(bucket)
	slices.Sort(keys)
	slices.Reverse(keys) // deterministic descending order so a missing sort in List is always caught
	return keys, err
}

type failingReader struct{ err error }

func (r failingReader) GetExisting(string, string, string) ([]byte, bool, error) {
	return nil, false, r.err
}

func newEdgeService(store *faultStore, now func() time.Time) *Service {
	return NewService(Dependencies{
		StateDir:        func() string { return "/state" },
		StatePath:       func(dir, key string) string { return dir + "/" + key + ".json" },
		OpenStore:       func(string) (stateport.Store, error) { return store, nil },
		ExistingRecords: memoryExistingReader{store: store.memoryStore},
		Now:             now,
	})
}

func newFaultStore() *faultStore {
	return &faultStore{memoryStore: &memoryStore{records: map[string][]byte{}}}
}

func TestReadRejectsInvalidKeyBeforeTouchingStorage(t *testing.T) {
	service := newEdgeService(newFaultStore(), nil)
	result, err := service.Read("../escape")
	if err == nil || result.OK || result.StateDir != "/state" || result.Path != "" {
		t.Fatalf("Read(invalid key) = %+v, %v", result, err)
	}
}

func TestReadMissingRecordReportsNotExistWithPath(t *testing.T) {
	service := newEdgeService(newFaultStore(), nil)
	result, err := service.Read("absent")
	if !errors.Is(err, fs.ErrNotExist) || result.OK || result.Path != "/state/absent.json" {
		t.Fatalf("Read(missing) = %+v, %v", result, err)
	}
	if pathErr, ok := errors.AsType[*fs.PathError](err); !ok || pathErr.Path != "/state/absent.json" {
		t.Fatalf("error is not a *fs.PathError carrying the record path: %v", err)
	}
}

func TestReadSurfacesReaderFailureWithoutRecord(t *testing.T) {
	boom := errors.New("reader down")
	store := newFaultStore()
	service := NewService(Dependencies{
		StateDir:        func() string { return "/state" },
		StatePath:       func(dir, key string) string { return dir + "/" + key + ".json" },
		OpenStore:       func(string) (stateport.Store, error) { return store, nil },
		ExistingRecords: failingReader{err: boom},
	})
	result, err := service.Read("k")
	if !errors.Is(err, boom) || result.OK || result.Path != "/state/k.json" {
		t.Fatalf("Read(reader failure) = %+v, %v", result, err)
	}
}

func TestReadRejectsStoredRecordThatFailsDecoding(t *testing.T) {
	store := newFaultStore()
	store.records[stateBucket+"/k"] = []byte(`{"schema_version":`)
	result, err := newEdgeService(store, nil).Read("k")
	if !errors.Is(err, statecontract.ErrInvalidState) || result.OK || result.Record.Key != "" {
		t.Fatalf("Read(corrupt) = %+v, %v", result, err)
	}
}

func TestDecodeRecordRejectsMalformedEnvelopes(t *testing.T) {
	valid := `{"schema_version":` + strconv.Itoa(statecontract.SchemaVersion) + `,"key":"k","content":"ab","updated_at":"2026-01-01T00:00:00Z","bytes":2}`
	if record, err := DecodeRecord("k", []byte(valid)); err != nil || record.Content != "ab" {
		t.Fatalf("baseline valid envelope rejected: %+v, %v", record, err)
	}
	cases := map[string]struct {
		key, data string
	}{
		"not json":         {"k", `nope`},
		"unknown field":    {"k", `{"schema_version":` + strconv.Itoa(statecontract.SchemaVersion) + `,"key":"k","content":"ab","updated_at":"x","bytes":2,"extra":1}`},
		"trailing value":   {"k", valid + `{}`},
		"key mismatch":     {"other", valid},
		"byte count drift": {"k", `{"schema_version":` + strconv.Itoa(statecontract.SchemaVersion) + `,"key":"k","content":"ab","updated_at":"x","bytes":3}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			record, err := DecodeRecord(tc.key, []byte(tc.data))
			if !errors.Is(err, statecontract.ErrInvalidState) || record != (statecontract.RecordEnvelope{}) {
				t.Fatalf("DecodeRecord = %+v, %v; want zero record and ErrInvalidState", record, err)
			}
		})
	}
}

func TestListSkipsInvalidKeysAndCorruptRecordsAndSortsByKey(t *testing.T) {
	store := newFaultStore()
	service := newEdgeService(store, func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) })
	for _, key := range []string{"zeta", "alpha"} {
		if _, err := service.Write(t.Context(), key, "v"); err != nil {
			t.Fatal(err)
		}
	}
	// Well-formed envelope under an invalid key: only the key guard can drop it.
	store.records[stateBucket+"/bad key"] = []byte(`{"schema_version":` + strconv.Itoa(statecontract.SchemaVersion) + `,"key":"bad key","content":"","updated_at":"2026-01-01T00:00:00Z","bytes":0}`)
	store.records[stateBucket+"/corrupt"] = []byte("garbage")

	listed, err := service.List()
	if err != nil || !listed.OK {
		t.Fatalf("List = %+v, %v", listed, err)
	}
	if !slices.Equal(listed.Keys, []string{"alpha", "zeta"}) || len(listed.Records) != 2 || listed.Records[0].Bytes != 1 {
		t.Fatalf("List must drop invalid/corrupt entries and sort: %+v", listed)
	}
}

func TestListReportsOpenAndEnumerationFailures(t *testing.T) {
	boom := errors.New("list failed")
	store := newFaultStore()
	store.listErr = boom
	result, err := newEdgeService(store, nil).List()
	if !errors.Is(err, boom) || result.OK || result.StateDir != "/state" {
		t.Fatalf("List(enumeration failure) = %+v, %v", result, err)
	}

	openErr := errors.New("open failed")
	service := NewService(Dependencies{
		StateDir:  func() string { return "/state" },
		OpenStore: func(string) (stateport.Store, error) { return nil, openErr },
	})
	if result, err := service.List(); !errors.Is(err, openErr) || result.OK {
		t.Fatalf("List(open failure) = %+v, %v", result, err)
	}
}

func TestWriteFailureModes(t *testing.T) {
	t.Run("invalid key", func(t *testing.T) {
		store := newFaultStore()
		result, err := newEdgeService(store, nil).Write(t.Context(), "a/b", "x")
		if err == nil || result.OK || len(store.records) != 0 {
			t.Fatalf("Write(invalid key) = %+v, %v records=%v", result, err, store.records)
		}
	})
	t.Run("open failure", func(t *testing.T) {
		openErr := errors.New("open failed")
		service := NewService(Dependencies{
			StateDir:  func() string { return "/state" },
			OpenStore: func(string) (stateport.Store, error) { return nil, openErr },
		})
		result, err := service.Write(t.Context(), "k", "x")
		if !errors.Is(err, openErr) || result.OK || result.StateDir != "/state" {
			t.Fatalf("Write(open failure) = %+v, %v", result, err)
		}
	})
	t.Run("mutation failure keeps path and reports not ok", func(t *testing.T) {
		boom := errors.New("disk full")
		store := newFaultStore()
		store.mutateErr = boom
		result, err := newEdgeService(store, nil).Write(t.Context(), "k", "x")
		if !errors.Is(err, boom) || result.OK || result.Path != "/state/k.json" {
			t.Fatalf("Write(mutation failure) = %+v, %v", result, err)
		}
		if len(store.records) != 0 {
			t.Fatalf("failed write must leave no record: %v", store.records)
		}
	})
}

func TestWriteDefaultsToWallClockWhenNowIsUnset(t *testing.T) {
	store := newFaultStore()
	before := time.Now().Add(-time.Second)
	result, err := newEdgeService(store, nil).Write(t.Context(), "k", "x")
	if err != nil {
		t.Fatal(err)
	}
	stamp, err := statecontract.ParseTime(result.Record.UpdatedAt)
	if err != nil || stamp.Before(before) || stamp.After(time.Now().Add(time.Second)) {
		t.Fatalf("UpdatedAt = %q (%v), want a current timestamp", result.Record.UpdatedAt, err)
	}
}

func TestWriteRecordFailureModes(t *testing.T) {
	envelope := statecontract.RecordEnvelope{SchemaVersion: statecontract.SchemaVersion, Key: "k", Content: "x", Bytes: 1}
	t.Run("invalid key", func(t *testing.T) {
		if path, err := newEdgeService(newFaultStore(), nil).WriteRecord(t.Context(), "/state", "..", envelope); err == nil || path != "" {
			t.Fatalf("WriteRecord(invalid key) = %q, %v", path, err)
		}
	})
	t.Run("open failure", func(t *testing.T) {
		openErr := errors.New("open failed")
		service := NewService(Dependencies{OpenStore: func(string) (stateport.Store, error) { return nil, openErr }})
		if path, err := service.WriteRecord(t.Context(), "/state", "k", envelope); !errors.Is(err, openErr) || path != "" {
			t.Fatalf("WriteRecord(open failure) = %q, %v", path, err)
		}
	})
	t.Run("mutation failure", func(t *testing.T) {
		boom := errors.New("disk full")
		store := newFaultStore()
		store.mutateErr = boom
		path, err := newEdgeService(store, nil).WriteRecord(t.Context(), "/state", "k", envelope)
		if !errors.Is(err, boom) || path != "/state/k.json" {
			t.Fatalf("WriteRecord(mutation failure) = %q, %v", path, err)
		}
	})
	t.Run("blank envelope key is rejected by validation", func(t *testing.T) {
		store := newFaultStore()
		blank := envelope
		blank.Key = ""
		// A blank key passes the mismatch guard; ValidateRecord then rejects it.
		if _, err := newEdgeService(store, nil).WriteRecord(t.Context(), "/state", "k", blank); !errors.Is(err, statecontract.ErrInvalidState) {
			t.Fatalf("WriteRecord(blank key) err = %v, want ErrInvalidState", err)
		}
		if len(store.records) != 0 {
			t.Fatalf("invalid envelope must not be stored: %v", store.records)
		}
	})
}

func TestDeleteRemovesRecordAndSurfacesFailures(t *testing.T) {
	store := newFaultStore()
	service := newEdgeService(store, nil)
	if _, err := service.Write(t.Context(), "k", "x"); err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(t.Context(), " k "); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := service.Read("k"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("record must be gone after Delete, Read err = %v", err)
	}
	if err := service.Delete(t.Context(), "a/b"); err == nil {
		t.Fatal("Delete(invalid key) must fail")
	}

	boom := errors.New("disk full")
	store.mutateErr = boom
	if err := service.Delete(t.Context(), "k"); !errors.Is(err, boom) {
		t.Fatalf("Delete(mutation failure) = %v", err)
	}

	openErr := errors.New("open failed")
	failing := NewService(Dependencies{
		StateDir:  func() string { return "/state" },
		OpenStore: func(string) (stateport.Store, error) { return nil, openErr },
	})
	if err := failing.Delete(t.Context(), "k"); !errors.Is(err, openErr) {
		t.Fatalf("Delete(open failure) = %v", err)
	}
}

func TestWithKeyLockRunsCallbackInsideSpanAndPropagatesItsError(t *testing.T) {
	store := newFaultStore()
	service := newEdgeService(store, nil)
	sentinel := errors.New("callback failed")
	called := false
	err := service.WithKeyLock(t.Context(), "/state", "k", func(context.Context) error {
		called = true
		return sentinel
	})
	if !errors.Is(err, sentinel) || !called || store.spanCalls != 1 {
		t.Fatalf("err=%v called=%v spans=%d", err, called, store.spanCalls)
	}

	err = service.WithKeyLock(t.Context(), "/state", "bad/key", func(context.Context) error {
		t.Error("callback must not run for an invalid key")
		return nil
	})
	if err == nil || store.spanCalls != 1 {
		t.Fatalf("invalid key: err=%v spans=%d", err, store.spanCalls)
	}

	openErr := errors.New("open failed")
	failing := NewService(Dependencies{OpenStore: func(string) (stateport.Store, error) { return nil, openErr }})
	err = failing.WithKeyLock(t.Context(), "/state", "k", func(context.Context) error {
		t.Error("callback must not run when the store cannot open")
		return nil
	})
	if !errors.Is(err, openErr) {
		t.Fatalf("WithKeyLock(open failure) = %v", err)
	}
}

func TestPrunePrefixLimitsDeletionToMatchingKeys(t *testing.T) {
	store := newFaultStore()
	now := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	service := newEdgeService(store, func() time.Time { return now })
	old := now.Add(-48 * time.Hour).Format(time.RFC3339Nano)
	for _, key := range []string{"loop-a", "loop-b", "other-a"} {
		envelope := statecontract.RecordEnvelope{SchemaVersion: statecontract.SchemaVersion, Key: key, Content: "x", UpdatedAt: old, Bytes: 1}
		if _, err := service.WriteRecord(t.Context(), "/state", key, envelope); err != nil {
			t.Fatal(err)
		}
	}
	result, err := service.PrunePrefix(t.Context(), "loop-", time.Hour, 0, true)
	if err != nil || !result.OK {
		t.Fatalf("PrunePrefix = %+v, %v", result, err)
	}
	if !slices.Equal(result.DeletedKeys, []string{"loop-a", "loop-b"}) || !slices.Equal(result.KeptKeys, []string{"other-a"}) {
		t.Fatalf("deleted=%v kept=%v", result.DeletedKeys, result.KeptKeys)
	}
	if _, err := service.Read("other-a"); err != nil {
		t.Fatalf("non-matching record must survive: %v", err)
	}
	if _, err := service.Read("loop-a"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("matching old record must be deleted: %v", err)
	}
}

func TestPrunePrefixHonorsMaxRecordsAndRejectsNegative(t *testing.T) {
	store := newFaultStore()
	now := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	service := newEdgeService(store, func() time.Time { return now })
	for i, key := range []string{"run-1", "run-2", "run-3"} {
		stamp := now.Add(-time.Duration(3-i) * time.Minute).Format(time.RFC3339Nano)
		envelope := statecontract.RecordEnvelope{SchemaVersion: statecontract.SchemaVersion, Key: key, Content: "x", UpdatedAt: stamp, Bytes: 1}
		if _, err := service.WriteRecord(t.Context(), "/state", key, envelope); err != nil {
			t.Fatal(err)
		}
	}
	result, err := service.PrunePrefix(t.Context(), "run-", time.Hour, 2, false)
	if err != nil || !result.DryRun {
		t.Fatalf("PrunePrefix(dry) = %+v, %v", result, err)
	}
	if !slices.Equal(result.DeletedKeys, []string{"run-1"}) || !slices.Equal(result.KeptKeys, []string{"run-2", "run-3"}) {
		t.Fatalf("max-records must drop only the oldest: deleted=%v kept=%v", result.DeletedKeys, result.KeptKeys)
	}

	rejected, err := service.PrunePrefix(t.Context(), "run-", time.Hour, -1, true)
	if err == nil || rejected.OK || rejected.Cutoff != "" {
		t.Fatalf("negative max records = %+v, %v", rejected, err)
	}
}

func TestPruneConfirmSurfacesStoreFailures(t *testing.T) {
	now := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	t.Run("delete failure", func(t *testing.T) {
		store := newFaultStore()
		service := newEdgeService(store, func() time.Time { return now })
		envelope := statecontract.RecordEnvelope{SchemaVersion: statecontract.SchemaVersion, Key: "old", Content: "x", UpdatedAt: now.Add(-48 * time.Hour).Format(time.RFC3339Nano), Bytes: 1}
		if _, err := service.WriteRecord(t.Context(), "/state", "old", envelope); err != nil {
			t.Fatal(err)
		}
		boom := errors.New("disk full")
		store.mutateErr = boom
		result, err := service.Prune(t.Context(), time.Hour, true)
		if !errors.Is(err, boom) || result.OK {
			t.Fatalf("Prune(delete failure) = %+v, %v", result, err)
		}
	})
	t.Run("open failure", func(t *testing.T) {
		openErr := errors.New("open failed")
		service := NewService(Dependencies{
			StateDir:  func() string { return "/state" },
			OpenStore: func(string) (stateport.Store, error) { return nil, openErr },
			Now:       func() time.Time { return now },
		})
		result, err := service.Prune(t.Context(), time.Hour, true)
		if !errors.Is(err, openErr) || result.OK || result.Cutoff == "" {
			t.Fatalf("Prune(open failure) = %+v, %v", result, err)
		}
	})
	t.Run("dry-run list failure", func(t *testing.T) {
		boom := errors.New("list failed")
		store := newFaultStore()
		store.listErr = boom
		result, err := newEdgeService(store, func() time.Time { return now }).Prune(t.Context(), time.Hour, false)
		if !errors.Is(err, boom) || result.OK {
			t.Fatalf("Prune(list failure) = %+v, %v", result, err)
		}
	})
}
