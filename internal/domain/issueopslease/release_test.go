package issueopslease

import (
	"errors"
	"testing"
	"time"
)

func releaseTestActor() Actor {
	return Actor{Host: "codex", SessionID: "session", AgentID: "agent", Process: &ProcessReceipt{PID: 7, StartedAt: "2026-07-30T00:00:00Z", Executable: "/codex"}}
}

func TestValidateReleaseRequiresActiveGenerationHolder(t *testing.T) {
	holder := releaseTestActor()
	lease := Lease{Generation: 4, Status: "active", Holder: &holder}
	request := ReleaseRequest{Generation: 4, Actor: holder, AuthorityVerified: true, CanonicalCWD: true}
	if err := ValidateRelease(lease, request); err != nil {
		t.Fatalf("holder release: %v", err)
	}
	tests := []struct {
		name   string
		lease  func(*Lease)
		edit   func(*ReleaseRequest)
		denied DenyCode
	}{
		{"authority not verified", nil, func(r *ReleaseRequest) { r.AuthorityVerified = false }, DenyLeaseAuthority},
		{"lease not active", func(l *Lease) { l.Status = "claimable" }, nil, DenyLeaseAuthority},
		{"stale generation", nil, func(r *ReleaseRequest) { r.Generation = 3 }, DenyLeaseAuthority},
		{"different session", nil, func(r *ReleaseRequest) { r.Actor.SessionID = "other" }, DenyLeaseAuthority},
		{"different agent", nil, func(r *ReleaseRequest) { r.Actor.AgentID = "other" }, DenyLeaseAuthority},
		{"different host", nil, func(r *ReleaseRequest) { r.Actor.Host = "claude" }, DenyLeaseAuthority},
		{"different process pid", nil, func(r *ReleaseRequest) {
			process := *r.Actor.Process
			process.PID++
			r.Actor.Process = &process
		}, DenyLeaseAuthority},
		{"missing process receipt", nil, func(r *ReleaseRequest) { r.Actor.Process = nil }, DenyLeaseAuthority},
		{"lease without holder", func(l *Lease) { l.Holder = nil }, nil, DenyLeaseAuthority},
		{"non-canonical cwd", nil, func(r *ReleaseRequest) { r.CanonicalCWD = false }, DenyCanonicalCWD},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			l, r := lease, request
			if test.lease != nil {
				test.lease(&l)
			}
			if test.edit != nil {
				test.edit(&r)
			}
			err := ValidateRelease(l, r)
			if err == nil || DenyCodeOf(err) != test.denied {
				t.Fatalf("error = %v code = %q, want code %q", err, DenyCodeOf(err), test.denied)
			}
		})
	}
}

func TestReleaseAuthorityNormalizesRequesterIdentity(t *testing.T) {
	holder := releaseTestActor()
	lease := Lease{Generation: 1, Status: "active", Holder: &holder}
	requester := releaseTestActor()
	requester.Host = "  CODEX "
	requester.SessionID = " session\n"
	requester.AgentID = "\tagent "
	request := ReleaseRequest{Generation: 1, Actor: requester, AuthorityVerified: true, CanonicalCWD: true}
	if err := ValidateRelease(lease, request); err != nil {
		t.Fatalf("host case and whitespace must not change identity: %v", err)
	}

	holder.SessionID = " session"
	if err := ValidateRelease(lease, request); DenyCodeOf(err) != DenyLeaseAuthority {
		t.Fatalf("persisted holder identity must be compared verbatim, got %v", err)
	}
}

func TestValidateReleaseDeniesZeroValueRequesterForHolderlessLease(t *testing.T) {
	lease := Lease{Generation: 1, Status: "active"}
	request := ReleaseRequest{Generation: 1, AuthorityVerified: true, CanonicalCWD: true}
	// A nil holder and an empty (non-nil) requester are different identities,
	// so the zero-value Actor in the request never matches a holderless lease.
	if err := ValidateRelease(lease, request); DenyCodeOf(err) != DenyLeaseAuthority {
		t.Fatalf("holderless lease released by a requester: %v", err)
	}
}

func TestApplyReleaseStampsUTCRFC3339Nano(t *testing.T) {
	zone := time.FixedZone("KST", 9*3600)
	outcome := ApplyRelease(time.Date(2026, 7, 30, 9, 0, 0, 123456789, zone))
	if outcome.Status != "released" || outcome.ReleasedAt != "2026-07-30T00:00:00.123456789Z" {
		t.Fatalf("outcome = %#v", outcome)
	}
}

func TestDenialExposesCodeAndCause(t *testing.T) {
	cause := errors.New("boom")
	err := Deny(DenyClaimToken, cause)
	if err.Error() != "claim_token: boom" {
		t.Fatalf("message = %q", err.Error())
	}
	if !errors.Is(err, cause) {
		t.Fatal("denial must unwrap to its cause")
	}
	if got := DenyCodeOf(err); got != DenyClaimToken {
		t.Fatalf("code = %q", got)
	}
	if got := DenyCodeOf(errors.New("plain")); got != "" {
		t.Fatalf("plain error code = %q, want empty", got)
	}
	if got := DenyCodeOf(nil); got != "" {
		t.Fatalf("nil error code = %q, want empty", got)
	}
}
