package issueopslease

import (
	"encoding/json"
	"testing"
)

func TestClaimTokenMatchesComparesSHA256Digest(t *testing.T) {
	// SHA-256 of "abc", independent of the production helper.
	const abc = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if !ClaimTokenMatches(abc, "abc") {
		t.Fatal("matching token rejected")
	}
	for name, token := range map[string]string{"wrong": "abd", "empty": "", "padded": "abc "} {
		if ClaimTokenMatches(abc, token) {
			t.Fatalf("%s token accepted", name)
		}
	}
	if ClaimTokenMatches("", "abc") || ClaimTokenMatches("ABC7816BF8F01CFEA414140DE5DAE2223B00361A396177A9CB410FF61F20015AD", "abc") {
		t.Fatal("empty or uppercase expected digest must not match the lowercase hex digest")
	}
}

func TestIsClaimRetryRequiresSameActiveHolderAndGeneration(t *testing.T) {
	holder := releaseTestActor()
	lease := Lease{Generation: 2, Status: "active", Holder: &holder}
	if !IsClaimRetry(lease, 2, releaseTestActor()) {
		t.Fatal("same holder at the active generation must be a retry")
	}
	other := releaseTestActor()
	other.SessionID = "other"
	if IsClaimRetry(lease, 2, other) {
		t.Fatal("different session treated as retry")
	}
	if IsClaimRetry(lease, 3, releaseTestActor()) {
		t.Fatal("different generation treated as retry")
	}
	claimable := lease
	claimable.Status = "claimable"
	if IsClaimRetry(claimable, 2, releaseTestActor()) {
		t.Fatal("non-active lease treated as retry")
	}
}

func TestValidateClaimAcceptsIdempotentRetryWithoutFreshProof(t *testing.T) {
	holder := releaseTestActor()
	lease := Lease{Generation: 2, Status: "active", Holder: &holder}
	// A retry by the current holder succeeds even with no authority, cwd, or token proof.
	if err := ValidateClaim(lease, ClaimRequest{Generation: 2, Actor: releaseTestActor()}); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if err := ValidateClaim(lease, ClaimRequest{Generation: 2, Actor: Actor{Host: "claude", SessionID: "x"}, AuthorityVerified: true, CanonicalCWD: true, TokenVerified: true}); DenyCodeOf(err) != DenyLeaseClaimable {
		t.Fatalf("a different actor must not claim an active lease: %v", err)
	}
}

func TestValidateClaimDeniesUnverifiedAuthorityBeforeOtherChecks(t *testing.T) {
	lease := Lease{Generation: 3, Status: "claimable"}
	err := ValidateClaim(lease, ClaimRequest{Generation: 3, Actor: releaseTestActor()})
	if DenyCodeOf(err) != DenyLeaseClaimable {
		t.Fatalf("code = %q, want %q (err=%v)", DenyCodeOf(err), DenyLeaseClaimable, err)
	}
}

func TestCleanupAbandonApplyingDetectsApplyingReceiptOnly(t *testing.T) {
	tests := []struct {
		name string
		raw  json.RawMessage
		want bool
	}{
		{"applying", json.RawMessage(`{"step":"applying","message":"x"}`), true},
		{"other step", json.RawMessage(`{"step":"close_pr"}`), false},
		{"missing step", json.RawMessage(`{}`), false},
		{"empty", nil, false},
		{"malformed", json.RawMessage(`{"step":`), false},
		{"non-object", json.RawMessage(`"applying"`), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CleanupAbandonApplying(test.raw); got != test.want {
				t.Fatalf("CleanupAbandonApplying(%s) = %v, want %v", test.raw, got, test.want)
			}
		})
	}
}
