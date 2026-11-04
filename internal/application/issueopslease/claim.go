// Package issueopslease is the application layer for the execution lease
// lifecycle: claim, release, and reseed. Services verify the caller through
// the authority port, ask the domain package for the transition decision, and
// persist the result through the repository ports defined here.
package issueopslease

import (
	"context"

	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
	authorityport "issueops/internal/port/authority"
)

// ClaimRequest asks to claim lease ID at Generation for Actor. Exactly one
// claim token source is required: TokenFile, or ClaimCurrentToken to use the
// token stored for the cycle. CWD must be inside the cycle's canonical root.
type ClaimRequest struct {
	ID                string
	Generation        uint64
	Actor             leasedomain.Actor
	Ancestry          []leasedomain.ProcessReceipt
	CWD               string
	TokenFile         string
	ClaimCurrentToken bool
}

// ClaimResult reports a claim. On success OK is true and Execution is the
// execution projection saved in the same transaction; on failure only ID is set.
type ClaimResult struct {
	OK        bool
	ID        string
	Execution leasecontract.Execution
}

// ClaimService claims execution leases. It verifies the actor, then delegates
// the atomic claim to its repository.
type ClaimService struct {
	repository claimRepository
	clock      clock
	verifier   authorityport.ActorVerifier
}

// NewClaimService returns a ClaimService that persists through repository, reads time from clock, and verifies callers with verifier.
func NewClaimService(repository claimRepository, clock clock, verifier authorityport.ActorVerifier) *ClaimService {
	return &ClaimService{repository: repository, clock: clock, verifier: verifier}
}

// Claim verifies the actor named in request and claims the lease through the
// repository. It returns a failed result carrying request.ID and the error
// when verification or the claim is rejected.
func (s *ClaimService) Claim(ctx context.Context, request ClaimRequest) (ClaimResult, error) {
	actor, err := resolveActor(ctx, request.Actor, request.Ancestry, s.verifier)
	if err != nil {
		return ClaimResult{ID: request.ID}, err
	}
	after, err := s.repository.Claim(ctx, ClaimRepositoryRequest{
		ID: request.ID, Generation: request.Generation, Actor: actor, CWD: request.CWD, TokenFile: request.TokenFile, ClaimCurrentToken: request.ClaimCurrentToken, Clock: s.clock,
	})
	if err != nil {
		return ClaimResult{ID: request.ID}, err
	}
	return ClaimResult{OK: true, ID: request.ID, Execution: after.Execution}, nil
}
