// Package issueopslease owns the pure state transitions and authority checks of the execution lease:
// claim, release, reseed, writerless recovery and validation of persisted lease records. It performs no I/O.
package issueopslease

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// DenyCode is the stable machine-readable reason a lease operation was denied.
type DenyCode string

// DenyLeaseAuthority and DenyCanonicalCWD are the denial codes for failed holder authority and for a
// non-canonical working directory.
const (
	DenyLeaseAuthority DenyCode = "lease_authority"
	DenyCanonicalCWD   DenyCode = "canonical_cwd"
)

type denial struct {
	Code  DenyCode
	Cause error
	// CanonicalRoot names the worktree a canonical_cwd denial expected, so
	// the public error can say where to rerun.
	CanonicalRoot string
}

func (e *denial) Error() string { return fmt.Sprintf("%s: %v", e.Code, e.Cause) }
func (e *denial) Unwrap() error { return e.Cause }

// Deny wraps cause in an error carrying code; extract the code with DenyCodeOf.
func Deny(code DenyCode, cause error) error { return &denial{Code: code, Cause: cause} }

// denyCWD is the canonical_cwd denial naming the expected worktree.
func denyCWD(root string, cause error) error {
	return &denial{Code: DenyCanonicalCWD, Cause: cause, CanonicalRoot: root}
}

// CanonicalRootOf returns the worktree a canonical_cwd denial expected.
func CanonicalRootOf(err error) string {
	if denial, ok := errors.AsType[*denial](err); ok && denial.Code == DenyCanonicalCWD {
		return denial.CanonicalRoot
	}
	return ""
}

// DenyCodeOf returns the DenyCode carried by err, or "" when err is not a lease denial.
func DenyCodeOf(err error) DenyCode {
	if denial, ok := errors.AsType[*denial](err); ok {
		return denial.Code
	}
	return ""
}

// Lease is the lease state the transition rules read: generation, status, current Holder and the SHA-256
// digest of the claim token.
type Lease struct {
	Generation       uint64
	Status           string
	Holder           *Actor
	ClaimTokenSHA256 string
}

// Actor identifies a lease holder by host, session, agent and the receipt of its process.
type Actor struct {
	Host      string
	SessionID string
	AgentID   string
	Process   *ProcessReceipt
}

// ProcessReceipt identifies a process by PID and start time so a reused PID is not mistaken for the
// original holder.
type ProcessReceipt struct {
	PID        int
	StartedAt  string
	Executable string
}

// ReleaseRequest is a release attempt: the generation and Actor releasing, plus the results of the
// authority and canonical-cwd checks performed by the caller.
type ReleaseRequest struct {
	Generation        uint64
	Actor             Actor
	AuthorityVerified bool
	CanonicalCWD      bool
	CanonicalRoot     string
}

// ValidateRelease checks that request may release lease: the lease must be active at the requested
// generation and held by the requesting actor, and the cwd must be canonical. Failures return a Deny error.
func ValidateRelease(lease Lease, request ReleaseRequest) error {
	if !request.AuthorityVerified || lease.Status != "active" || lease.Generation != request.Generation || !sameActor(lease.Holder, &request.Actor) {
		return Deny(DenyLeaseAuthority, fmt.Errorf("requester is not the active generation holder"))
	}
	if !request.CanonicalCWD {
		return denyCWD(request.CanonicalRoot, fmt.Errorf("release cwd is not canonical"))
	}
	return nil
}

// ApplyRelease returns the released lease state stamped with now (UTC, RFC 3339).
func ApplyRelease(now time.Time) ReleaseOutcome {
	return ReleaseOutcome{Status: "released", ReleasedAt: now.UTC().Format(time.RFC3339Nano)}
}

// ReleaseOutcome is the lease state produced by a release: the status and the RFC 3339 UTC timestamp.
type ReleaseOutcome struct {
	Status     string
	ReleasedAt string
}

func sameActor(left, right *Actor) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return strings.EqualFold(strings.TrimSpace(left.Host), strings.TrimSpace(right.Host)) && left.SessionID == strings.TrimSpace(right.SessionID) && left.AgentID == strings.TrimSpace(right.AgentID) && sameProcess(left.Process, right.Process)
}
func sameProcess(left, right *ProcessReceipt) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
