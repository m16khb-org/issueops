package issueopsapp

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	auditadapter "issueops/internal/adapter/audit"
	issueopscontract "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"

	leaseinbound "issueops/internal/adapter/inbound/issueopslease"
	"issueops/internal/adapter/issueops"
	leaseoutbound "issueops/internal/adapter/outbound/issueopslease"
	"issueops/internal/adapter/outbound/sqlstore"
	leaseapp "issueops/internal/application/issueopslease"
	"issueops/internal/port"
)

func issueOpsClaimHandler(ctx context.Context, stateRoot string, request issueops.ExecutionClaimRequest, deps issueops.ExecutionClaimDependencies) (issueops.ExecutionResult, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return issueops.ExecutionResult{ID: request.ID}, err
	}
	preflight := leaseoutbound.NewClaimContextPreflight(db, func(ctx context.Context, repo, issueURL string) (leaseoutbound.IssueSnapshot, error) {
		record, err := issueops.ReadIssueOps(stateRoot, request.ID)
		if err != nil {
			return leaseoutbound.IssueSnapshot{}, err
		}
		providerName, err := issueOpsClaimProviderName(record)
		if err != nil {
			return leaseoutbound.IssueSnapshot{}, err
		}
		if deps.ReadIssue == nil {
			return leaseoutbound.IssueSnapshot{}, fmt.Errorf("remote issue snapshot reader is unavailable for the Orca claim")
		}
		snapshot, err := deps.ReadIssue(ctx, providerName, port.ExecutionIssueSnapshotRequest{Repo: repo, URL: issueURL})
		if err != nil {
			return leaseoutbound.IssueSnapshot{}, err
		}
		return leaseoutbound.IssueSnapshot{URL: snapshot.URL, Body: snapshot.Body}, nil
	})
	service := leaseapp.NewClaimService(leaseoutbound.NewSQLiteRepository(db), leaseoutbound.UTCClock{}, leaseoutbound.InspectNativeProcess, preflight)
	result, err := leaseinbound.NewClaimHandler(service)(ctx, stateRoot, request, deps)
	if err != nil {
		return result, err
	}
	// The committed lease is the authority. Observation is deliberately
	// best-effort after that commit, so an audit failure cannot turn a
	// successful CAS into a false claim failure.
	_ = observeSuccessfulIssueOpsClaim(stateRoot, result)
	return result, nil
}

func observeSuccessfulIssueOpsClaim(stateRoot string, result issueops.ExecutionResult) error {
	execution := result.Execution
	if !result.OK || execution.Lease.Status != issueopscontract.LeaseStatusActive || execution.Lease.Holder == nil {
		return nil
	}
	if execution.Orca == nil {
		return observeSuccessfulManualIssueOpsClaim(stateRoot, result)
	}
	binding := execution.Orca
	callKind := "dispatch"
	if binding.OwnerHost == "omo" {
		callKind = "prompt"
	}
	lineageID := strings.Join([]string{
		"generation", strconv.FormatUint(execution.Lease.Generation, 10),
		"prompt", strings.TrimSpace(binding.OwnerPromptSHA256),
		"material", strings.TrimSpace(binding.ContextPacketSHA256),
		"call", callKind,
	}, ":")
	folded, decisions, err := auditadapter.FoldHandoffDeliveryAuditObservationsForAt(stateRoot, result.ID, lineageID)
	if err != nil {
		return err
	}
	for _, decision := range decisions {
		if !decision.Accepted {
			return fmt.Errorf("handoff delivery claim evidence is invalid: %s", strings.Join(decision.RejectReasons, "; "))
		}
	}
	observation, ok := folded[result.ID+"\x00"+lineageID]
	if !ok {
		return nil
	}
	holder := *execution.Lease.Holder
	if observation.SourceGeneration != execution.Lease.Generation || observation.ExpectedOwnerHost != holder.Host ||
		observation.Launcher.RuntimeID != binding.RuntimeID ||
		(binding.TerminalPTYID != "" && observation.Target.TerminalID != binding.TerminalPTYID) {
		return fmt.Errorf("handoff delivery claim does not match the committed lease")
	}
	return appendSuccessfulOwnerClaim(stateRoot, observation, execution.Lease)
}

func observeSuccessfulManualIssueOpsClaim(stateRoot string, result issueops.ExecutionResult) error {
	if result.Execution.Mode != issueopscontract.ExecutionModeDirect {
		return nil
	}
	observations, err := auditadapter.ReadHandoffDeliveryAuditObservationsAt(stateRoot)
	if err != nil {
		return err
	}
	candidates := make([]issueopscontract.IssueOpsHandoffDeliveryObservation, 0, 1)
	for _, observation := range observations {
		if observation.LifecycleID == result.ID && observation.SourceGeneration == result.Execution.Lease.Generation &&
			strings.HasPrefix(observation.LineageID, handoffDeliveryManualLineagePrefix) && observation.ExpectedOwnerHost == result.Execution.Lease.Holder.Host {
			candidates = append(candidates, observation)
		}
	}
	folded, decisions := issueopsdomain.FoldHandoffDeliveryObservations(candidates)
	for _, decision := range decisions {
		if !decision.Accepted {
			return fmt.Errorf("manual handoff delivery claim evidence is invalid: %s", strings.Join(decision.RejectReasons, "; "))
		}
	}
	if len(folded) == 0 {
		return nil
	}
	if len(folded) != 1 {
		return fmt.Errorf("manual handoff delivery claim evidence is ambiguous")
	}
	holderProcess := result.Execution.Lease.Holder.SessionProcess
	for _, observation := range folded {
		process := observation.Target.Process
		accepted := observation.InputAccepted.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved ||
			observation.NativeTurnObserved.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved
		if accepted && holderProcess != nil && process != nil && process.PID == holderProcess.PID &&
			process.StartedAt == holderProcess.StartedAt && process.Executable == holderProcess.Executable {
			return appendSuccessfulOwnerClaim(stateRoot, observation, result.Execution.Lease)
		}
	}
	return nil
}

func appendSuccessfulOwnerClaim(stateRoot string, observation issueopscontract.IssueOpsHandoffDeliveryObservation, lease issueopscontract.WriteLease) error {
	holder := *lease.Holder
	claimedAt := strings.TrimSpace(lease.ClaimedAt)
	observation.UpdatedAt = claimedAt
	observation.OwnerActor = &holder
	observation.OwnerClaimed = issueopscontract.IssueOpsHandoffDeliveryState{
		Status: issueopscontract.IssueOpsHandoffDeliveryStateObserved, ObservedAt: claimedAt,
		Evidence: issueopscontract.IssueOpsHandoffDeliveryEvidenceIssueOpsClaim,
	}
	observation.OwnerClaim = issueopscontract.IssueOpsHandoffDeliveryOwnerClaim{
		Claimed: true, Generation: lease.Generation, Actor: holder, ClaimedAt: claimedAt,
	}
	if err := issueopsdomain.ValidateHandoffDeliveryObservation(observation); err != nil {
		return err
	}
	_, err := auditadapter.AuditHandoffDeliveryObservationAt(stateRoot, observation)
	return err
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
