package gates

import (
	"bytes"
	"errors"
	"testing"
	"time"

	gatescontract "issueops/internal/contract/gates"
	gatesdomain "issueops/internal/domain/gates"
)

type memoryLedgerStore struct {
	data   []byte
	writes int
}

func (store *memoryLedgerStore) Discover(string) ([]string, error) { return []string{"gates.md"}, nil }
func (store *memoryLedgerStore) Read(string) ([]byte, error) {
	return append([]byte(nil), store.data...), nil
}
func (store *memoryLedgerStore) WritePreservingMode(_ string, data []byte) error {
	store.writes++
	store.data = append([]byte(nil), data...)
	return nil
}

type runnerStub struct {
	calls   int
	outcome gatesdomain.CheckOutcome
}

func (runner *runnerStub) Run(string, string, gatescontract.CheckRequest, gatesdomain.Gate) gatesdomain.CheckOutcome {
	runner.calls++
	return runner.outcome
}

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC) }

func TestServiceStatusOnlyKeepsLedgerBytesAndSkipsRunner(t *testing.T) {
	original := []byte("# Gates: task\r\n- [ ] G1: proof\r\n  CHECK: printf ok\r\n  EXPECT: ok\r\n  EVIDENCE: pending\r\n")
	store := &memoryLedgerStore{data: append([]byte(nil), original...)}
	runner := &runnerStub{outcome: gatesdomain.CheckOutcome{Passed: true, Evidence: "ok"}}
	result, err := (Service{Store: store, Runner: runner, Clock: fixedClock{}}).Check(gatescontract.CheckRequest{CWD: "/repo", StatusOnly: true})
	if err != nil || result.TotalUnmet != 1 || runner.calls != 0 || store.writes != 0 || !bytes.Equal(original, store.data) {
		t.Fatalf("status-only result=%+v err=%v calls=%d writes=%d", result, err, runner.calls, store.writes)
	}
}

func TestServiceRunsUnmetGateAndPreservesUnmanagedText(t *testing.T) {
	store := &memoryLedgerStore{data: []byte("# Gates: task\ncustom note\n- [ ] G1: proof\n  CHECK: printf ok\n  EXPECT: ok\n  EVIDENCE: pending\n")}
	runner := &runnerStub{outcome: gatesdomain.CheckOutcome{Passed: true, Evidence: "ok", AuditLogID: "audit-1"}}
	result, err := (Service{Store: store, Runner: runner, Clock: fixedClock{}}).Check(gatescontract.CheckRequest{CWD: "/repo"})
	if err != nil || !result.Complete || runner.calls != 1 || store.writes != 1 || !bytes.Contains(store.data, []byte("custom note")) || !bytes.Contains(store.data, []byte("- [x] G1")) {
		t.Fatalf("check result=%+v err=%v calls=%d writes=%d data=%q", result, err, runner.calls, store.writes, store.data)
	}
}

func TestServiceDeniedGateLeavesLedgerUnchanged(t *testing.T) {
	original := []byte("# Gates: task\n- [ ] G1: proof\n  CHECK: printf ok\n  EVIDENCE: pending\n")
	store := &memoryLedgerStore{data: append([]byte(nil), original...)}
	runner := &runnerStub{outcome: gatesdomain.CheckOutcome{PolicyDenied: true, CheckError: "check denied by policy", AuditLogID: "audit-1"}}
	result, err := (Service{Store: store, Runner: runner, Clock: fixedClock{}}).Check(gatescontract.CheckRequest{CWD: "/repo"})
	if err != nil || result.TotalUnmet != 1 || runner.calls != 1 || store.writes != 0 || !bytes.Equal(original, store.data) || len(result.Warnings) != 1 || !result.Files[0].Gates[0].PolicyDenied {
		t.Fatalf("denied result=%+v err=%v calls=%d writes=%d", result, err, runner.calls, store.writes)
	}
}

func TestServiceRequiresDiscoveredLedger(t *testing.T) {
	store := emptyDiscoveryStore{}
	_, err := (Service{Store: store, Clock: fixedClock{}}).Check(gatescontract.CheckRequest{CWD: "/repo"})
	if !errors.Is(err, gatescontract.ErrNoGateFiles) {
		t.Fatalf("empty discovery error = %v", err)
	}
}

type emptyDiscoveryStore struct{}

func (emptyDiscoveryStore) Discover(string) ([]string, error)        { return nil, nil }
func (emptyDiscoveryStore) Read(string) ([]byte, error)              { panic("unexpected read") }
func (emptyDiscoveryStore) WritePreservingMode(string, []byte) error { panic("unexpected write") }

func (*memoryLedgerStore) ExistsFile(string) bool       { panic("unexpected existence check") }
func (*memoryLedgerStore) Create(string, []byte) error  { panic("unexpected create") }
func (emptyDiscoveryStore) ExistsFile(string) bool      { panic("unexpected existence check") }
func (emptyDiscoveryStore) Create(string, []byte) error { panic("unexpected create") }
