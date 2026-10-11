package issueopslease

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	model "issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
)

func claimServiceActor() leasedomain.Actor {
	return leasedomain.Actor{Host: "codex", SessionID: "claim-session", Process: &leasedomain.ProcessReceipt{PID: 9, StartedAt: "2026-07-30T00:00:00Z", Executable: "/codex"}}
}

func TestClaimServiceRefusesWithoutPreflight(t *testing.T) {
	service := NewClaimService(claimRepositoryFunc(func(context.Context, ClaimRepositoryRequest) (RepositoryResult, error) {
		t.Fatal("repository reached without a context preflight")
		return RepositoryResult{}, nil
	}), fixedClaimClock{}, nil, nil)
	result, err := service.Claim(t.Context(), ClaimRequest{ID: "io-claim"})
	if err == nil || !strings.Contains(err.Error(), "claim context preflight is required") || result.OK || result.ID != "io-claim" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestClaimServiceStopsBeforeRepositoryWhenPreflightOrActorProofFails(t *testing.T) {
	actor := claimServiceActor()
	live := leaseVerifier(func(_ context.Context, receipt leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
		return "live", receipt, nil
	})
	okPreflight := claimPreflightFunc(func(context.Context, ClaimPreflightRequest) (RecordValidator, error) {
		return func(Record) error { return nil }, nil
	})
	tests := []struct {
		name      string
		preflight ClaimContextPreflight
		verifier  func() *ClaimService
		want      string
	}{
		{
			name: "preflight rejection",
			preflight: claimPreflightFunc(func(context.Context, ClaimPreflightRequest) (RecordValidator, error) {
				return nil, errors.New("sealed context drifted")
			}),
			want: "sealed context drifted",
		},
		{
			name:      "actor not verifiable",
			preflight: okPreflight,
			verifier: func() *ClaimService {
				return NewClaimService(nil, fixedClaimClock{}, leaseVerifier(func(context.Context, leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
					return "", leasedomain.ProcessReceipt{}, errors.New("process inspection failed")
				}), okPreflight)
			},
			want: "process inspection failed",
		},
		{
			name:      "no verifier configured",
			preflight: okPreflight,
			verifier:  func() *ClaimService { return NewClaimService(nil, fixedClaimClock{}, nil, okPreflight) },
			want:      "native actor verifier is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := claimRepositoryFunc(func(context.Context, ClaimRepositoryRequest) (RepositoryResult, error) {
				t.Fatal("repository reached after a failed gate")
				return RepositoryResult{}, nil
			})
			service := NewClaimService(repository, fixedClaimClock{}, live, tt.preflight)
			if tt.verifier != nil {
				service = tt.verifier()
				service.repository = repository
			}
			result, err := service.Claim(t.Context(), ClaimRequest{ID: "io-claim", Generation: 3, Actor: actor, Ancestry: []leasedomain.ProcessReceipt{*actor.Process}})
			if err == nil || !strings.Contains(err.Error(), tt.want) || result.OK || result.ID != "io-claim" {
				t.Fatalf("result=%+v err=%v want %q", result, err, tt.want)
			}
		})
	}
}

func TestClaimServiceSurfacesRepositoryRefusalAndReturnsClaimedExecutionOnSuccess(t *testing.T) {
	actor := claimServiceActor()
	verifier := leaseVerifier(func(_ context.Context, receipt leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
		return "live", receipt, nil
	})
	preflight := claimPreflightFunc(func(context.Context, ClaimPreflightRequest) (RecordValidator, error) {
		return func(Record) error { return nil }, nil
	})
	refused := errors.New("lease not claimable")
	request := ClaimRequest{ID: "io-claim", Generation: 3, Actor: actor, Ancestry: []leasedomain.ProcessReceipt{*actor.Process}, TokenFile: "/tmp/token"}

	failing := NewClaimService(claimRepositoryFunc(func(context.Context, ClaimRepositoryRequest) (RepositoryResult, error) {
		return RepositoryResult{}, refused
	}), fixedClaimClock{}, verifier, preflight)
	if result, err := failing.Claim(t.Context(), request); !errors.Is(err, refused) || result.OK || result.Execution.Lease.Status != "" {
		t.Fatalf("refusal result=%+v err=%v", result, err)
	}

	var seen ClaimRepositoryRequest
	claimed := leasecontract.Execution{Lease: leasecontract.Lease{Generation: 3, Status: "active"}}
	succeeding := NewClaimService(claimRepositoryFunc(func(_ context.Context, got ClaimRepositoryRequest) (RepositoryResult, error) {
		seen = got
		return RepositoryResult{Execution: claimed}, nil
	}), fixedClaimClock{}, verifier, preflight)
	result, err := succeeding.Claim(t.Context(), request)
	if err != nil || !result.OK || result.ID != "io-claim" || !reflect.DeepEqual(result.Execution, claimed) {
		t.Fatalf("success result=%+v err=%v", result, err)
	}
	if seen.TokenFile != "/tmp/token" || seen.Generation != 3 || seen.ID != "io-claim" || seen.ClaimCurrentToken {
		t.Fatalf("repository request lost the claim selector: %+v", seen)
	}
}

type failingClaimTransaction struct {
	*claimTransactionStub
	loadErr      error
	readTokenErr error
}

func (f *failingClaimTransaction) Load(id string) (Record, error) {
	if f.loadErr != nil {
		return Record{}, f.loadErr
	}
	return f.claimTransactionStub.Load(id)
}

func (f *failingClaimTransaction) ReadToken(record leasecontract.Record, path string) (string, error) {
	if f.readTokenErr != nil {
		f.events = append(f.events, "token")
		return "", f.readTokenErr
	}
	return f.claimTransactionStub.ReadToken(record, path)
}

func claimTransactionRequest() ClaimRepositoryRequest {
	return ClaimRepositoryRequest{
		ID: "io-claim", Generation: 3, Actor: leasedomain.Actor{Host: "codex", SessionID: "owner"}, CWD: "/repo",
		ClaimCurrentToken: true, ValidateRecord: func(Record) error { return nil }, Clock: claimTransactionClock{},
	}
}

func TestClaimWithinTransactionRefusesBeforeAnyMutation(t *testing.T) {
	loadErr := errors.New("record unreadable")
	readErr := errors.New("token unreadable")
	validatorErr := errors.New("sealed context drifted")
	tests := []struct {
		name       string
		build      func() *failingClaimTransaction
		mutate     func(*ClaimRepositoryRequest)
		check      func(error) bool
		wantEvents []string
	}{
		{
			name: "load failure",
			build: func() *failingClaimTransaction {
				return &failingClaimTransaction{claimTransactionStub: claimableTransactionStub(), loadErr: loadErr}
			},
			check:      func(err error) bool { return errors.Is(err, loadErr) },
			wantEvents: nil,
		},
		{
			name: "execution not prepared",
			build: func() *failingClaimTransaction {
				stub := claimableTransactionStub()
				stub.record.Stable.Execution = nil
				return &failingClaimTransaction{claimTransactionStub: stub}
			},
			check:      func(err error) bool { return errors.Is(err, leasecontract.ErrExecutionNotPrepared) },
			wantEvents: []string{"load"},
		},
		{
			name: "pending cleanup attempt blocks a fresh claim",
			build: func() *failingClaimTransaction {
				stub := claimableTransactionStub()
				stub.record.Stable.CleanupAttempt = &model.IssueOpsCleanupAttempt{Operation: "finish", Token: strings.Repeat("a", 64), StartedAt: "2026-09-29T00:00:00Z"}
				return &failingClaimTransaction{claimTransactionStub: stub}
			},
			check: func(err error) bool {
				return leasedomain.DenyCodeOf(err) == leasedomain.DenyLeaseClaimable && strings.Contains(err.Error(), "cleanup finish")
			},
			wantEvents: []string{"load"},
		},
		{
			name: "missing record validator",
			build: func() *failingClaimTransaction {
				return &failingClaimTransaction{claimTransactionStub: claimableTransactionStub()}
			},
			mutate: func(r *ClaimRepositoryRequest) { r.ValidateRecord = nil },
			check: func(err error) bool {
				return err != nil && strings.Contains(err.Error(), "claim record validator is required")
			},
			wantEvents: []string{"load", "cwd"},
		},
		{
			name: "validator rejection precedes token read",
			build: func() *failingClaimTransaction {
				return &failingClaimTransaction{claimTransactionStub: claimableTransactionStub()}
			},
			mutate:     func(r *ClaimRepositoryRequest) { r.ValidateRecord = func(Record) error { return validatorErr } },
			check:      func(err error) bool { return errors.Is(err, validatorErr) },
			wantEvents: []string{"load", "cwd"},
		},
		{
			name: "no token selector",
			build: func() *failingClaimTransaction {
				return &failingClaimTransaction{claimTransactionStub: claimableTransactionStub()}
			},
			mutate: func(r *ClaimRepositoryRequest) { r.ClaimCurrentToken = false },
			check: func(err error) bool {
				return err != nil && strings.Contains(err.Error(), "exactly one claim token selector")
			},
			wantEvents: []string{"load", "cwd"},
		},
		{
			name: "token unreadable",
			build: func() *failingClaimTransaction {
				return &failingClaimTransaction{claimTransactionStub: claimableTransactionStub(), readTokenErr: readErr}
			},
			check:      func(err error) bool { return errors.Is(err, readErr) },
			wantEvents: []string{"load", "cwd", "token"},
		},
		{
			name: "missing clock after a matching token",
			build: func() *failingClaimTransaction {
				return &failingClaimTransaction{claimTransactionStub: claimableTransactionStub()}
			},
			mutate:     func(r *ClaimRepositoryRequest) { r.Clock = nil },
			check:      func(err error) bool { return err != nil && strings.Contains(err.Error(), "claim clock is required") },
			wantEvents: []string{"load", "cwd", "token"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := tt.build()
			request := claimTransactionRequest()
			if tt.mutate != nil {
				tt.mutate(&request)
			}
			_, err := ClaimWithinTransaction(t.Context(), stub, request)
			if !tt.check(err) {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(stub.events, tt.wantEvents) {
				t.Fatalf("events=%v want %v", stub.events, tt.wantEvents)
			}
		})
	}
}

func TestClaimWithinTransactionRecordsHolderProcessReceipt(t *testing.T) {
	stub := claimableTransactionStub()
	request := claimTransactionRequest()
	request.Actor = leasedomain.Actor{Host: "codex", SessionID: "owner", AgentID: "agent-1", Process: &leasedomain.ProcessReceipt{PID: 42, StartedAt: "t0", Executable: "/bin/codex"}}
	request.Clock = fixedClaimClock{at: time.Date(2026, 9, 25, 1, 2, 3, 0, time.UTC)}
	result, err := ClaimWithinTransaction(t.Context(), stub, request)
	if err != nil {
		t.Fatal(err)
	}
	holder := result.Execution.Lease.Holder
	want := &leasecontract.Actor{Host: "codex", SessionID: "owner", AgentID: "agent-1", SessionProcess: &leasecontract.ProcessReceipt{PID: 42, StartedAt: "t0", Executable: "/bin/codex"}}
	if !reflect.DeepEqual(holder, want) || result.Execution.Lease.ClaimedAt == "" {
		t.Fatalf("holder=%+v lease=%+v", holder, result.Execution.Lease)
	}
}
