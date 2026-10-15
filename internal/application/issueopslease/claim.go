package issueopslease

import (
	"context"

	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
	authorityport "issueops/internal/port/authority"
)

type ClaimRequest struct {
	ID                string
	Generation        uint64
	Actor             leasedomain.Actor
	Ancestry          []leasedomain.ProcessReceipt
	CWD               string
	TokenFile         string
	ClaimCurrentToken bool
}

type ClaimResult struct {
	OK        bool
	ID        string
	Execution leasecontract.Execution
}

type ClaimService struct {
	repository ClaimRepository
	clock      Clock
	verifier   authorityport.ActorVerifier
}

func NewClaimService(repository ClaimRepository, clock Clock, verifier authorityport.ActorVerifier) *ClaimService {
	return &ClaimService{repository: repository, clock: clock, verifier: verifier}
}

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
