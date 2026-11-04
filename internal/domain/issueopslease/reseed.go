package issueopslease

import (
	"fmt"
	"strings"
	"time"
)

// DenyLeaseGeneration is the denial code for a stale or zero lease generation.
const (
	DenyLeaseGeneration DenyCode = "lease_generation"
	denyReseedLease     DenyCode = "reseed_lease"
)

// ReseedRequest asks to replace a lease: the generation the caller observed, whether the cwd is
// canonical, and the free-text reason recorded with the replacement.
type ReseedRequest struct {
	ExpectedGeneration uint64
	CanonicalCWD       bool
	Reason             string
}

// ReseedOutcome is the fresh claimable lease state produced by a reseed: the next generation with no
// holder or token, and when and why it replaced the previous lease.
type ReseedOutcome struct {
	Generation        uint64
	Status            string
	Holder            *Actor
	ClaimTokenSHA256  string
	ReplacedAt        string
	ReplacementReason string
}

// ValidateReseed checks that lease may be reseeded: the expected generation must be current, the cwd
// canonical, and the lease released or claimable with no holder. Failures return a Deny error.
func ValidateReseed(lease Lease, request ReseedRequest) error {
	if err := ValidateReseedGeneration(lease, request.ExpectedGeneration); err != nil {
		return err
	}
	if !request.CanonicalCWD {
		return Deny(DenyCanonicalCWD, fmt.Errorf("execution replace cwd must be source_root or the canonical worktree"))
	}
	if (lease.Status != "released" && lease.Status != "claimable") || lease.Holder != nil {
		return Deny(denyReseedLease, fmt.Errorf("reseed requires a released or claimable lease"))
	}
	return nil
}

// ValidateReseedGeneration returns a DenyLeaseGeneration error unless expected is non-zero and equals
// the lease's current generation.
func ValidateReseedGeneration(lease Lease, expected uint64) error {
	if expected == 0 || lease.Generation != expected {
		return Deny(DenyLeaseGeneration, fmt.Errorf("stale lease generation: current=%d expected=%d", lease.Generation, expected))
	}
	return nil
}

// ApplyReseed returns a claimable lease at the next generation with no holder or claim token, stamped
// with now (UTC, RFC 3339) and the trimmed request reason.
func ApplyReseed(now time.Time, lease Lease, request ReseedRequest) ReseedOutcome {
	return ReseedOutcome{
		Generation:        lease.Generation + 1,
		Status:            "claimable",
		Holder:            nil,
		ClaimTokenSHA256:  "",
		ReplacedAt:        now.UTC().Format(time.RFC3339Nano),
		ReplacementReason: strings.TrimSpace(request.Reason),
	}
}
