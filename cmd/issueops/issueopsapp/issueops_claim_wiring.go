package issueopsapp

import (
	"context"
	"fmt"
	"strings"

	issueopscontract "issueops/internal/contract/issueops"

	leaseinbound "issueops/internal/adapter/inbound/issueopslease"
	"issueops/internal/adapter/issueops"
	leaseoutbound "issueops/internal/adapter/outbound/issueopslease"
	"issueops/internal/adapter/outbound/sqlstore"
	leaseapp "issueops/internal/application/issueopslease"
	"issueops/internal/port"
)

func issueOpsClaimHandler(ctx context.Context, stateRoot string, request issueopscontract.ExecutionClaimRequest, deps issueopscontract.ExecutionClaimDependencies) (issueopscontract.ExecutionResult, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return issueopscontract.ExecutionResult{ID: request.ID}, err
	}
	preflight := leaseapp.NewSealedClaimContext(leaseoutbound.NewClaimContextReader(db), func(ctx context.Context, repo, issueURL string) (leaseapp.IssueSnapshot, error) {
		record, err := issueops.ReadIssueOps(stateRoot, request.ID)
		if err != nil {
			return leaseapp.IssueSnapshot{}, err
		}
		providerName, err := issueOpsClaimProviderName(record)
		if err != nil {
			return leaseapp.IssueSnapshot{}, err
		}
		if deps.ReadIssue == nil {
			return leaseapp.IssueSnapshot{}, fmt.Errorf("remote issue snapshot reader is unavailable for the Orca claim")
		}
		snapshot, err := deps.ReadIssue(ctx, providerName, port.ExecutionIssueSnapshotRequest{Repo: repo, URL: issueURL})
		if err != nil {
			return leaseapp.IssueSnapshot{}, err
		}
		return leaseapp.IssueSnapshot{URL: snapshot.URL, Body: snapshot.Body}, nil
	}, leaseoutbound.FilesystemPathMatcher{})
	service := leaseapp.NewClaimService(leaseoutbound.NewSQLiteRepository(db), leaseoutbound.UTCClock{}, issueOpsActorVerifier(), preflight)
	result, err := leaseinbound.NewClaimHandler(service)(ctx, stateRoot, request, deps)
	if err != nil {
		return result, err
	}
	// The committed lease is the authority. Observation is deliberately
	// best-effort after that commit, so an audit failure cannot turn a
	// successful CAS into a false claim failure.
	_ = newHandoffDeliveryService(stateRoot).ObserveClaim(result)
	return result, nil
}

func issueOpsClaimProviderName(record issueopscontract.IssueOpsRecord) (string, error) {
	if record.BranchPrepare != nil {
		switch providerName := strings.ToLower(strings.TrimSpace(record.BranchPrepare.Provider)); providerName {
		case "github", "gitlab":
			return providerName, nil
		}
	}
	return "", fmt.Errorf("linked issue provider is unavailable")
}
