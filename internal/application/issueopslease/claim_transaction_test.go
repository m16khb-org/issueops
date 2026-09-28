package issueopslease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	model "issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
)

type claimTransactionStub struct {
	record     Record
	events     []string
	result     RepositoryResult
	token      string
	persistErr error
}

func (stub *claimTransactionStub) Load(string) (Record, error) {
	stub.events = append(stub.events, "load")
	return stub.record, nil
}
func (stub *claimTransactionStub) CanonicalCWD(string, string) bool {
	stub.events = append(stub.events, "cwd")
	return true
}
func (stub *claimTransactionStub) CurrentTokenPath(leasecontract.Record) string { return "/token" }
func (stub *claimTransactionStub) ReadToken(leasecontract.Record, string) (string, error) {
	stub.events = append(stub.events, "token")
	if stub.token != "" {
		return stub.token, nil
	}
	return "claim-token", nil
}
func (stub *claimTransactionStub) Persist(_ context.Context, record leasecontract.Record) (RepositoryResult, error) {
	stub.events = append(stub.events, "persist")
	if stub.persistErr != nil {
		return RepositoryResult{}, stub.persistErr
	}
	stub.result = RepositoryResult{Record: Record{ID: record.ID, Lease: record.Execution.Lease, Stable: record}, Execution: *record.Execution}
	return stub.result, nil
}
func (stub *claimTransactionStub) RemoveToken(string) { stub.events = append(stub.events, "remove") }

func TestClaimWithinTransactionOrdersValidationTokenAndAtomicPersist(t *testing.T) {
	stub := claimableTransactionStub()
	request := ClaimRepositoryRequest{
		ID: "io-claim", Generation: 3, Actor: leasedomain.Actor{Host: "codex", SessionID: "owner"},
		CWD: "/repo", ClaimCurrentToken: true,
		ValidateRecord: func(Record) error { stub.events = append(stub.events, "validate"); return nil },
		Clock:          claimTransactionClock{},
	}
	result, err := ClaimWithinTransaction(context.Background(), stub, request)
	if err != nil || result.Execution.Lease.Status != "active" || result.Execution.Lease.ClaimTokenSHA256 != "" {
		t.Fatalf("claim result = %+v, %v", result, err)
	}
	want := []string{"load", "cwd", "validate", "token", "persist", "remove"}
	if !reflect.DeepEqual(stub.events, want) {
		t.Fatalf("claim order = %v, want %v", stub.events, want)
	}
}

func TestClaimWithinTransactionPreservesRetryAndCleanupFence(t *testing.T) {
	stub := claimableTransactionStub()
	stub.record.Lease.Status = "active"
	stub.record.Lease.Holder = &leasecontract.Actor{Host: "codex", SessionID: "owner"}
	stub.record.Stable.Execution.Lease = stub.record.Lease
	request := ClaimRepositoryRequest{ID: "io-claim", Generation: 3, Actor: leasedomain.Actor{Host: "codex", SessionID: "owner"}}
	if _, err := ClaimWithinTransaction(context.Background(), stub, request); err != nil || !reflect.DeepEqual(stub.events, []string{"load"}) {
		t.Fatalf("retry result = %v, events = %v", err, stub.events)
	}
	stub = claimableTransactionStub()
	stub.record.Stable.CleanupAbandonFailure = json.RawMessage(`{"step":"applying"}`)
	_, err := ClaimWithinTransaction(context.Background(), stub, request)
	if leasedomain.DenyCodeOf(err) != leasedomain.DenyLeaseClaimable || !reflect.DeepEqual(stub.events, []string{"load"}) {
		t.Fatalf("cleanup fence = %v, events = %v", err, stub.events)
	}
}

func TestClaimWithinTransactionRejectsAmbiguousSelectorBeforeTokenRead(t *testing.T) {
	stub := claimableTransactionStub()
	request := ClaimRepositoryRequest{
		ID: "io-claim", Generation: 3, Actor: leasedomain.Actor{Host: "codex", SessionID: "owner"}, CWD: "/repo",
		TokenFile: "/token", ClaimCurrentToken: true, ValidateRecord: func(Record) error { return nil },
	}
	_, err := ClaimWithinTransaction(context.Background(), stub, request)
	if err == nil || !strings.Contains(err.Error(), "exactly one claim token selector") || !reflect.DeepEqual(stub.events, []string{"load", "cwd"}) {
		t.Fatalf("selector = %v, events = %v", err, stub.events)
	}
}

func TestClaimWithinTransactionRejectsStaleGenerationBeforeValidatorAndToken(t *testing.T) {
	stub := claimableTransactionStub()
	request := ClaimRepositoryRequest{
		ID: "io-claim", Generation: 2, Actor: leasedomain.Actor{Host: "codex", SessionID: "owner"}, CWD: "/repo",
		ClaimCurrentToken: true, ValidateRecord: func(Record) error { t.Fatal("stale claim validated record"); return nil },
	}
	_, err := ClaimWithinTransaction(context.Background(), stub, request)
	if leasedomain.DenyCodeOf(err) != leasedomain.DenyLeaseClaimable || !reflect.DeepEqual(stub.events, []string{"load", "cwd"}) {
		t.Fatalf("stale generation = %v, events = %v", err, stub.events)
	}
}

func TestClaimWithinTransactionRejectsTokenMismatchAndKeepsTokenOnPersistFailure(t *testing.T) {
	request := ClaimRepositoryRequest{
		ID: "io-claim", Generation: 3, Actor: leasedomain.Actor{Host: "codex", SessionID: "owner"}, CWD: "/repo",
		ClaimCurrentToken: true, ValidateRecord: func(Record) error { return nil }, Clock: claimTransactionClock{},
	}
	stub := claimableTransactionStub()
	stub.token = "wrong-token"
	_, err := ClaimWithinTransaction(context.Background(), stub, request)
	if leasedomain.DenyCodeOf(err) != leasedomain.DenyClaimToken || !reflect.DeepEqual(stub.events, []string{"load", "cwd", "token"}) {
		t.Fatalf("mismatch = %v, events = %v", err, stub.events)
	}
	stub = claimableTransactionStub()
	stub.persistErr = errors.New("apply failed")
	_, err = ClaimWithinTransaction(context.Background(), stub, request)
	if err == nil || !reflect.DeepEqual(stub.events, []string{"load", "cwd", "token", "persist"}) {
		t.Fatalf("persist failure = %v, events = %v", err, stub.events)
	}
}

func claimableTransactionStub() *claimTransactionStub {
	sum := sha256.Sum256([]byte("claim-token"))
	lease := leasecontract.Lease{Generation: 3, Status: "claimable", ClaimTokenSHA256: hex.EncodeToString(sum[:])}
	stable := leasecontract.Record{ID: "io-claim", Execution: &leasecontract.Execution{Lease: lease}}
	return &claimTransactionStub{record: Record{ID: stable.ID, CanonicalRoot: "/repo", Lease: lease, Stable: stable}}
}

type claimTransactionClock struct{}

func (claimTransactionClock) Now() time.Time { return time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC) }

func TestClaimRetryCannotBypassFinishAttempt(t *testing.T) {
	stub := claimableTransactionStub()
	stub.record.Lease.Status = "active"
	stub.record.Lease.Holder = &leasecontract.Actor{Host: "codex", SessionID: "owner"}
	stub.record.Stable.Execution.Lease = stub.record.Lease
	stub.record.Stable.CleanupFinishAttempt = &model.IssueOpsCleanupFinishAttempt{Token: strings.Repeat("a", 64), StartedAt: "2026-09-29T00:00:00Z"}
	_, err := ClaimWithinTransaction(context.Background(), stub, ClaimRepositoryRequest{ID: "io-claim", Generation: 3, Actor: leasedomain.Actor{Host: "codex", SessionID: "owner"}})
	if err == nil || !strings.Contains(err.Error(), "cleanup finish") || !reflect.DeepEqual(stub.events, []string{"load"}) {
		t.Fatalf("retry bypassed finish: err=%v events=%v", err, stub.events)
	}
}
