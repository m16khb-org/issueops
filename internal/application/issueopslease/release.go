package issueopslease

import (
	"context"
	"fmt"

	model "issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
	"issueops/internal/domain/issueopslease"
	authorityport "issueops/internal/port/authority"
)

// ReleaseRequest asks to release lease ID at Generation. Actor and Ancestry identify the caller, and CWD must be inside the cycle's canonical root.
type ReleaseRequest struct {
	ID         string
	Generation uint64
	Actor      issueopslease.Actor
	Ancestry   []issueopslease.ProcessReceipt
	CWD        string
}

// ReleaseResult reports a release. On success OK is true and Execution is the projection saved by the update; on failure only ID is set.
type ReleaseResult struct {
	OK        bool
	ID        string
	Execution leasecontract.Execution
}

// ReleaseService releases execution leases held by a verified actor.
type ReleaseService struct {
	repository Repository
	clock      clock
	verifier   authorityport.ActorVerifier
	paths      canonicalPathMatcher
}

// NewReleaseService returns a ReleaseService that updates through repository, reads time from clock, verifies callers with verifier, and matches working directories with paths.
func NewReleaseService(repository Repository, clock clock, verifier authorityport.ActorVerifier, paths canonicalPathMatcher) *ReleaseService {
	return &ReleaseService{repository: repository, clock: clock, verifier: verifier, paths: paths}
}

// Release verifies the actor, validates holder, generation, and canonical cwd
// against the stored lease, and records the lease as released. It returns a
// failed result carrying request.ID and the error when any step is rejected.
func (s *ReleaseService) Release(ctx context.Context, request ReleaseRequest) (ReleaseResult, error) {
	actor, err := resolveActor(ctx, request.Actor, request.Ancestry, s.verifier)
	if err != nil {
		return ReleaseResult{ID: request.ID}, err
	}
	validate := func(before Record) error {
		canonicalCWD := s.paths != nil && s.paths.Matches(request.CWD, before.CanonicalRoot)
		release := issueopslease.ReleaseRequest{Generation: request.Generation, Actor: actor, AuthorityVerified: true, CanonicalCWD: canonicalCWD, CanonicalRoot: before.CanonicalRoot}
		return issueopslease.ValidateRelease(toDomainLease(before.Lease), release)
	}
	after, err := s.repository.Update(ctx, request.ID, validate, func(before Record) (Record, error) {
		outcome := issueopslease.ApplyRelease(s.clock.Now())
		before.Lease.Status = outcome.Status
		before.Lease.Holder = nil
		before.Lease.ClaimTokenSHA256 = ""
		before.Lease.ReleasedAt = outcome.ReleasedAt
		return before, nil
	})
	if err != nil {
		return ReleaseResult{ID: request.ID}, err
	}
	return ReleaseResult{OK: true, ID: request.ID, Execution: after.Execution}, nil
}

func toDomainLease(lease leasecontract.Lease) issueopslease.Lease {
	result := issueopslease.Lease{Generation: lease.Generation, Status: lease.Status, ClaimTokenSHA256: lease.ClaimTokenSHA256}
	if lease.Holder != nil {
		result.Holder = &issueopslease.Actor{Host: lease.Holder.Host, SessionID: lease.Holder.SessionID, AgentID: lease.Holder.AgentID}
		if lease.Holder.SessionProcess != nil {
			result.Holder.Process = &issueopslease.ProcessReceipt{PID: lease.Holder.SessionProcess.PID, StartedAt: lease.Holder.SessionProcess.StartedAt, Executable: lease.Holder.SessionProcess.Executable}
		}
	}
	return result
}

// resolveActor proves the caller through the shared verifier (native ancestry
// or a bound capability). Proof of identity is not lease ownership: the lease
// transition still checks holder, generation, token, and canonical cwd.
func resolveActor(ctx context.Context, actor issueopslease.Actor, ancestry []issueopslease.ProcessReceipt, verifier authorityport.ActorVerifier) (issueopslease.Actor, error) {
	if verifier == nil {
		return issueopslease.Actor{}, fmt.Errorf("native actor verifier is required")
	}
	native := model.NativeActor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.Process != nil {
		native.SessionProcess = &model.NativeProcessReceipt{PID: actor.Process.PID, StartedAt: actor.Process.StartedAt, Executable: actor.Process.Executable}
	}
	for _, receipt := range ancestry {
		native.ProcessAncestry = append(native.ProcessAncestry, model.NativeProcessReceipt{PID: receipt.PID, StartedAt: receipt.StartedAt, Executable: receipt.Executable})
	}
	verified, err := verifier.Verify(ctx, native)
	if err != nil {
		return issueopslease.Actor{}, err
	}
	identity := verified.Identity
	result := issueopslease.Actor{Host: identity.Host, SessionID: identity.SessionID, AgentID: identity.AgentID}
	if identity.SessionProcess != nil {
		result.Process = &issueopslease.ProcessReceipt{PID: identity.SessionProcess.PID, StartedAt: identity.SessionProcess.StartedAt, Executable: identity.SessionProcess.Executable}
	}
	return result, nil
}
