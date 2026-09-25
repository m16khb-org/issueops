package issueopslease

import (
	"context"
	"fmt"

	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
)

func ClaimWithinTransaction(ctx context.Context, transaction ClaimTransaction, request ClaimRepositoryRequest) (RepositoryResult, error) {
	before, err := transaction.Load(request.ID)
	if err != nil {
		return RepositoryResult{}, err
	}
	if before.Stable.Execution == nil {
		return RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, leasecontract.ErrExecutionNotPrepared)
	}
	lease := toDomainLease(before.Lease)
	if leasedomain.IsClaimRetry(lease, request.Generation, request.Actor) {
		return RepositoryResult{Record: before, Execution: *before.Stable.Execution}, nil
	}
	if leasedomain.CleanupAbandonApplying(before.Stable.CleanupAbandonFailure) {
		return RepositoryResult{}, leasedomain.Deny(leasedomain.DenyLeaseClaimable, fmt.Errorf("lease is fenced by cleanup abandon apply"))
	}
	canonicalCWD := transaction.CanonicalCWD(request.CWD, before.CanonicalRoot)
	claim := leasedomain.ClaimRequest{
		Generation: request.Generation, Actor: request.Actor, AuthorityVerified: true, CanonicalCWD: canonicalCWD, TokenVerified: true,
	}
	if err := leasedomain.ValidateClaim(lease, claim); err != nil {
		return RepositoryResult{}, err
	}
	if request.ValidateRecord == nil {
		return RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("claim record validator is required"))
	}
	if err := request.ValidateRecord(before); err != nil {
		return RepositoryResult{}, err
	}
	if (request.TokenFile == "") == !request.ClaimCurrentToken {
		return RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("exactly one claim token selector is required"))
	}
	tokenPath := request.TokenFile
	if request.ClaimCurrentToken {
		tokenPath = transaction.CurrentTokenPath(before.Stable)
	}
	token, err := transaction.ReadToken(before.Stable, tokenPath)
	if err != nil {
		return RepositoryResult{}, err
	}
	if !leasedomain.ClaimTokenMatches(before.Lease.ClaimTokenSHA256, token) {
		claim.TokenVerified = false
		return RepositoryResult{}, leasedomain.ValidateClaim(lease, claim)
	}
	if request.Clock == nil {
		return RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("claim clock is required"))
	}
	outcome := leasedomain.ApplyClaim(request.Clock.Now(), request.Actor)
	after := before.Stable
	after.Execution.Lease.Status = outcome.Status
	after.Execution.Lease.Holder = claimContractActor(*outcome.Holder)
	after.Execution.Lease.ClaimTokenSHA256 = ""
	after.Execution.Lease.ClaimedAt = outcome.ClaimedAt
	after.Execution.Lease.ReleasedAt = outcome.ReleasedAt
	result, err := transaction.Persist(ctx, after)
	if err != nil {
		return RepositoryResult{}, err
	}
	transaction.RemoveToken(tokenPath)
	return result, nil
}

func claimContractActor(actor leasedomain.Actor) *leasecontract.Actor {
	result := &leasecontract.Actor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.Process != nil {
		result.SessionProcess = &leasecontract.ProcessReceipt{PID: actor.Process.PID, StartedAt: actor.Process.StartedAt, Executable: actor.Process.Executable}
	}
	return result
}
