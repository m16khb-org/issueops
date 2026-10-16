package issueopslease

import (
	"context"
	"errors"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
)

func TestNilHandlersFailClosedWithConfiguredErrors(t *testing.T) {
	ctx := context.Background()

	_, claimErr := NewClaimHandler(nil)(ctx, "", issueopscontract.ExecutionClaimRequest{ID: "io-1"})
	if !errors.Is(claimErr, issueopscontract.ErrClaimHandlerUnavailable) {
		t.Fatalf("claim handler err = %v, want ErrClaimHandlerUnavailable", claimErr)
	}

	_, reseedErr := NewReseedHandler(nil, nil)(ctx, "", issueopscontract.ExecutionReseedRequest{ID: "io-1"})
	if !errors.Is(reseedErr, issueopscontract.ErrReseedHandlerUnavailable) {
		t.Fatalf("reseed handler err = %v, want ErrReseedHandlerUnavailable", reseedErr)
	}
}

func TestPublicClaimErrorMapsDenialsToStableMessages(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantSub string
	}{
		{"not claimable", leasedomain.Deny(leasedomain.DenyLeaseClaimable, errors.New("x")), "lease is not claimable at generation 5"},
		{"canonical cwd", leasedomain.Deny(leasedomain.DenyCanonicalCWD, errors.New("x")), "claim cwd must be the canonical worktree"},
		{"claim token", leasedomain.Deny(leasedomain.DenyClaimToken, errors.New("x")), "claim token does not match"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapped := publicClaimError(tt.err, 5, "io-demo")
			if mapped == nil || !strings.Contains(mapped.Error(), tt.wantSub) {
				t.Fatalf("mapped = %v, want containing %q", mapped, tt.wantSub)
			}
		})
	}

	cause := errors.New("state store offline")
	if mapped := publicClaimError(&leasecontract.Failure{Code: leasecontract.FailurePersistence, Cause: cause}, 5, "io-demo"); !errors.Is(mapped, cause) {
		t.Fatalf("failure cause lost: %v", mapped)
	}
	plain := errors.New("boom")
	if mapped := publicClaimError(plain, 5, "io-demo"); !errors.Is(mapped, plain) {
		t.Fatalf("plain error must pass through: %v", mapped)
	}
}

func TestPublicReseedErrorUnwrapsDenials(t *testing.T) {
	cause := errors.New("generation moved")

	reseeded := publicReseedError(leasedomain.Deny(leasedomain.DenyCanonicalCWD, cause))
	if !errors.Is(reseeded, cause) {
		t.Fatalf("reseed denial unwrap lost cause: %v", reseeded)
	}
	failureCause := errors.New("inventory fingerprint drift")
	if mapped := publicReseedError(&leasecontract.Failure{Cause: failureCause}); !errors.Is(mapped, failureCause) {
		t.Fatalf("reseed failure cause lost: %v", mapped)
	}
	plain := errors.New("boom")
	if mapped := publicReseedError(plain); !errors.Is(mapped, plain) {
		t.Fatalf("reseed plain error must pass through: %v", mapped)
	}
}

func TestToCoreLeaseCopiesHolderAndProcessSafely(t *testing.T) {
	bare := toCoreLease(leasecontract.Lease{Generation: 2, Status: "released"})
	if bare.Holder != nil || bare.Generation != 2 || bare.Status != issueopscontract.LeaseStatus("released") {
		t.Fatalf("bare lease mapping drifted: %+v", bare)
	}

	domain := leasecontract.Lease{
		Generation: 3,
		Status:     "active",
		Holder: &leasecontract.Actor{
			Host:           "codex",
			SessionID:      "s-1",
			AgentID:        "a-1",
			SessionProcess: &leasecontract.ProcessReceipt{PID: 7, StartedAt: "t", Executable: "/bin/codex"},
		},
	}
	core := toCoreLease(domain)
	if core.Holder == nil || core.Holder.Host != "codex" || core.Holder.AgentID != "a-1" {
		t.Fatalf("holder mapping drifted: %+v", core.Holder)
	}
	if core.Holder.SessionProcess == nil || core.Holder.SessionProcess.PID != 7 {
		t.Fatalf("process receipt mapping drifted: %+v", core.Holder.SessionProcess)
	}

	noProcess := domain
	noProcess.Holder.SessionProcess = nil
	if mapped := toCoreLease(noProcess); mapped.Holder.SessionProcess != nil {
		t.Fatalf("nil process must stay nil: %+v", mapped.Holder)
	}
}

func TestHandlersFailClosedWhenOnlyServiceIsMissing(t *testing.T) {
	ctx := context.Background()

	_, reseedErr := NewReseedHandler(nil, func(string, uint64) string { return "next" })(ctx, "", issueopscontract.ExecutionReseedRequest{ID: "io-1"})
	if !errors.Is(reseedErr, issueopscontract.ErrReseedHandlerUnavailable) {
		t.Fatalf("reseed handler err = %v, want ErrReseedHandlerUnavailable", reseedErr)
	}
}

func TestPublicReseedErrorReturnsDenialCauseItself(t *testing.T) {
	cause := errors.New("generation moved")
	if mapped := publicReseedError(leasedomain.Deny(leasedomain.DenyClaimToken, cause)); mapped != cause {
		t.Fatalf("reseed denial must surface its bare cause, got %v", mapped)
	}
}
