package issueopsapp

import (
	"context"

	issueopscontract "issueops/internal/contract/issueops"

	leaseinbound "issueops/internal/adapter/inbound/issueopslease"
	leaseoutbound "issueops/internal/adapter/outbound/issueopslease"
	"issueops/internal/adapter/outbound/sqlstore"
	leaseapp "issueops/internal/application/issueopslease"
)

func issueOpsClaimHandler(ctx context.Context, stateRoot string, request issueopscontract.ExecutionClaimRequest) (issueopscontract.ExecutionResult, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return issueopscontract.ExecutionResult{ID: request.ID}, err
	}
	service := leaseapp.NewClaimService(leaseoutbound.NewSQLiteRepository(db), leaseoutbound.UTCClock{}, issueOpsActorVerifier())
	result, err := leaseinbound.NewClaimHandler(service)(ctx, stateRoot, request)
	if err != nil {
		return result, err
	}
	// The committed lease is the authority. Observation is deliberately
	// best-effort after that commit, so an audit failure cannot turn a
	// successful CAS into a false claim failure.
	_ = newHandoffDeliveryService(stateRoot).ObserveClaim(result)
	return result, nil
}
