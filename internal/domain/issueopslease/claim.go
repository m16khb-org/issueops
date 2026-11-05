package issueopslease

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// CleanupAbandonApplying reports whether the raw cleanup receipt is valid JSON whose step is "applying",
// meaning an abandon cleanup was started but not finished.
func CleanupAbandonApplying(raw json.RawMessage) bool {
	var receipt struct {
		Step string `json:"step"`
	}
	return len(raw) > 0 && json.Unmarshal(raw, &receipt) == nil && receipt.Step == "applying"
}

// ClaimTokenMatches reports whether the SHA-256 hex digest of token equals expectedDigest.
func ClaimTokenMatches(expectedDigest, token string) bool {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:]) == expectedDigest
}

// DenyLeaseClaimable and DenyClaimToken are the denial codes returned when claiming a lease.
const (
	DenyLeaseClaimable DenyCode = "lease_claimable"
	DenyClaimToken     DenyCode = "claim_token"
)

// ClaimRequest is a claim attempt: the generation the actor believes is current, the claiming Actor, and
// the results of the authority, canonical-cwd and claim-token checks performed by the caller.
type ClaimRequest struct {
	Generation        uint64
	Actor             Actor
	AuthorityVerified bool
	CanonicalCWD      bool
	CanonicalRoot     string
	TokenVerified     bool
}

// ClaimOutcome is the lease state produced by a successful claim: the status, the new Holder and the
// RFC 3339 UTC claim timestamp.
type ClaimOutcome struct {
	Status     string
	Holder     *Actor
	ClaimedAt  string
	ReleasedAt string
}

// IsClaimRetry reports whether lease is already active at generation for the same actor, so a repeated
// claim can be treated as idempotent.
func IsClaimRetry(lease Lease, generation uint64, actor Actor) bool {
	return lease.Status == "active" && lease.Generation == generation && sameActor(lease.Holder, &actor)
}

// ValidateClaim checks that request may claim lease. An identical active claim is accepted as a retry.
// Otherwise the lease must be claimable at the requested generation with verified authority, a
// canonical cwd and a matching claim token; failures return a Deny error with the matching DenyCode.
func ValidateClaim(lease Lease, request ClaimRequest) error {
	if lease.Status == "active" && lease.Generation == request.Generation && sameActor(lease.Holder, &request.Actor) {
		return nil
	}
	if !request.AuthorityVerified || lease.Status != "claimable" || lease.Generation != request.Generation {
		return Deny(DenyLeaseClaimable, fmt.Errorf("lease is not claimable at generation %d", request.Generation))
	}
	if !request.CanonicalCWD {
		return denyCWD(request.CanonicalRoot, fmt.Errorf("claim cwd is not canonical"))
	}
	if !request.TokenVerified {
		return Deny(DenyClaimToken, fmt.Errorf("claim token does not match the current generation"))
	}
	return nil
}

// ApplyClaim returns the active lease state with actor as holder, claimed at now (UTC, RFC 3339).
func ApplyClaim(now time.Time, actor Actor) ClaimOutcome {
	holder := actor
	return ClaimOutcome{
		Status:    "active",
		Holder:    &holder,
		ClaimedAt: now.UTC().Format(time.RFC3339Nano),
	}
}
