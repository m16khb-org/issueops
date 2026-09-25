package looprun

import (
	"context"
	"errors"
	"io/fs"
	"reflect"
	"testing"

	loopcontract "issueops/internal/contract/looprun"
)

type memoryStore struct {
	loop   loopcontract.LoopRun
	exists bool
	events []string
}

func (store *memoryStore) WithLock(_ context.Context, _ string, fn func(context.Context) error) error {
	store.events = append(store.events, "lock")
	return fn(context.Background())
}

func (store *memoryStore) Read(_ string) (loopcontract.LoopRun, error) {
	store.events = append(store.events, "read")
	if !store.exists {
		return loopcontract.LoopRun{}, fs.ErrNotExist
	}
	return store.loop, nil
}

func (store *memoryStore) Write(loop loopcontract.LoopRun) (loopcontract.LoopRun, error) {
	store.events = append(store.events, "write")
	store.loop, store.exists = loop, true
	return loop, nil
}

type fixedIdentity struct{}

func (fixedIdentity) NormalizeRepo(string) (string, error)  { return "/repo", nil }
func (fixedIdentity) NormalizeID(id string) (string, error) { return id, nil }
func (fixedIdentity) NewID(string, string) string           { return "loop-id" }

type fixedClock struct{}

func (fixedClock) Now() string { return "2026-09-25T00:00:00Z" }

func TestServiceStartRecordsAttemptAndStopsWithinLocks(t *testing.T) {
	store := &memoryStore{}
	service := Service{Store: store, Identity: fixedIdentity{}, Clock: fixedClock{}, SchemaVersion: 1}
	started, err := service.Start(loopcontract.StartLoopRequest{Repo: "/repo", Name: "test", Goal: "finish"})
	if err != nil || started.Status != "active" || started.MaxAttempts != 5 {
		t.Fatalf("start: %+v, %v", started, err)
	}
	tried, err := service.RecordAttempt(started.ID, loopcontract.RecordAttemptRequest{Verdict: "pass", Evidence: []string{"test passed"}})
	if err != nil || len(tried.Attempts) != 1 {
		t.Fatalf("attempt: %+v, %v", tried, err)
	}
	stopped, err := service.Stop(started.ID, true, "")
	if err != nil || stopped.Status != "succeeded" {
		t.Fatalf("stop: %+v, %v", stopped, err)
	}
	want := []string{"lock", "read", "write", "lock", "read", "write", "lock", "read", "write"}
	if !reflect.DeepEqual(store.events, want) {
		t.Fatalf("events = %v, want %v", store.events, want)
	}
}

func TestServiceRejectsInvalidAttemptBeforeLockAndTerminalRestartWithoutWrite(t *testing.T) {
	store := &memoryStore{exists: true, loop: loopcontract.LoopRun{Status: "stopped"}}
	service := Service{Store: store, Identity: fixedIdentity{}, Clock: fixedClock{}, SchemaVersion: 1}
	if _, err := service.RecordAttempt("loop-id", loopcontract.RecordAttemptRequest{Verdict: "invalid"}); err == nil {
		t.Fatal("invalid attempt was accepted")
	}
	if len(store.events) != 0 {
		t.Fatalf("invalid attempt touched store: %v", store.events)
	}
	if _, err := service.Start(loopcontract.StartLoopRequest{Repo: "/repo", Name: "test", Goal: "finish"}); err == nil || err.Error() != "loop_terminal" {
		t.Fatalf("terminal restart error = %v", err)
	}
	if !reflect.DeepEqual(store.events, []string{"lock", "read"}) {
		t.Fatalf("terminal restart wrote store: %v", store.events)
	}
}

func TestServicePreservesReadFailure(t *testing.T) {
	store := &errorStore{err: errors.New("read failed")}
	service := Service{Store: store, Identity: fixedIdentity{}, Clock: fixedClock{}, SchemaVersion: 1}
	if _, err := service.Start(loopcontract.StartLoopRequest{Repo: "/repo", Name: "test", Goal: "finish"}); err == nil || err.Error() != "read failed" {
		t.Fatalf("read error = %v", err)
	}
}

type errorStore struct{ err error }

func (store *errorStore) WithLock(_ context.Context, _ string, fn func(context.Context) error) error {
	return fn(context.Background())
}
func (store *errorStore) Read(string) (loopcontract.LoopRun, error) {
	return loopcontract.LoopRun{}, store.err
}
func (store *errorStore) Write(loopcontract.LoopRun) (loopcontract.LoopRun, error) {
	panic("unexpected write")
}
