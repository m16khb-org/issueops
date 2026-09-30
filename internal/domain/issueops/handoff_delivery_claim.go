package issueops

import (
	"fmt"
	"strconv"
	"strings"

	issueopscontract "issueops/internal/contract/issueops"
)

type HandoffDeliveryClaimQuery struct {
	Observe     bool
	Manual      bool
	LifecycleID string
	LineageID   string
}

func HandoffDeliveryClaimQueryFor(result issueopscontract.ExecutionResult) HandoffDeliveryClaimQuery {
	execution := result.Execution
	if !result.OK || execution.Lease.Status != issueopscontract.LeaseStatusActive || execution.Lease.Holder == nil {
		return HandoffDeliveryClaimQuery{}
	}
	if execution.Orca == nil {
		return HandoffDeliveryClaimQuery{Observe: execution.Mode == issueopscontract.ExecutionModeDirect, Manual: true, LifecycleID: result.ID}
	}
	binding := execution.Orca
	callKind := "dispatch"
	if binding.OwnerHost == "omo" {
		callKind = "prompt"
	}
	return HandoffDeliveryClaimQuery{Observe: true, LifecycleID: result.ID, LineageID: strings.Join([]string{"generation", strconv.FormatUint(execution.Lease.Generation, 10), "prompt", strings.TrimSpace(binding.OwnerPromptSHA256), "material", strings.TrimSpace(binding.ContextPacketSHA256), "call", callKind}, ":")}
}

func OwnerClaimHandoffDeliveryObservation(result issueopscontract.ExecutionResult, observations []issueopscontract.IssueOpsHandoffDeliveryObservation) (*issueopscontract.IssueOpsHandoffDeliveryObservation, error) {
	query := HandoffDeliveryClaimQueryFor(result)
	if !query.Observe {
		return nil, nil
	}
	if query.Manual {
		return manualOwnerClaimObservation(result, observations)
	}
	folded, decisions := FoldHandoffDeliveryObservations(observations)
	for _, decision := range decisions {
		if !decision.Accepted {
			return nil, fmt.Errorf("handoff delivery claim evidence is invalid: %s", strings.Join(decision.RejectReasons, "; "))
		}
	}
	observation, ok := folded[result.ID+"\x00"+query.LineageID]
	if !ok {
		return nil, nil
	}
	execution := result.Execution
	binding := execution.Orca
	holder := *execution.Lease.Holder
	if observation.SourceGeneration != execution.Lease.Generation || observation.ExpectedOwnerHost != holder.Host || observation.Launcher.RuntimeID != binding.RuntimeID || (binding.TerminalPTYID != "" && observation.Target.TerminalID != binding.TerminalPTYID) {
		return nil, fmt.Errorf("handoff delivery claim does not match the committed lease")
	}
	return claimedHandoffDeliveryObservation(observation, execution.Lease)
}

func manualOwnerClaimObservation(result issueopscontract.ExecutionResult, observations []issueopscontract.IssueOpsHandoffDeliveryObservation) (*issueopscontract.IssueOpsHandoffDeliveryObservation, error) {
	candidates := make([]issueopscontract.IssueOpsHandoffDeliveryObservation, 0, 1)
	for _, observation := range observations {
		if observation.LifecycleID == result.ID && result.Execution.Lease.Generation > 1 &&
			observation.SourceGeneration == result.Execution.Lease.Generation-1 &&
			strings.HasPrefix(observation.LineageID, handoffDeliveryManualLineagePrefix) && observation.ExpectedOwnerHost == result.Execution.Lease.Holder.Host {
			candidates = append(candidates, observation)
		}
	}
	folded, decisions := FoldHandoffDeliveryObservations(candidates)
	for _, decision := range decisions {
		if !decision.Accepted {
			return nil, fmt.Errorf("manual handoff delivery claim evidence is invalid: %s", strings.Join(decision.RejectReasons, "; "))
		}
	}
	if len(folded) == 0 {
		return nil, nil
	}
	if len(folded) != 1 {
		return nil, fmt.Errorf("manual handoff delivery claim evidence is ambiguous")
	}
	holderProcess := result.Execution.Lease.Holder.SessionProcess
	for _, observation := range folded {
		process := observation.Target.Process
		accepted := observation.InputAccepted.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved ||
			observation.NativeTurnObserved.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved
		if accepted && holderProcess != nil && process != nil && process.PID == holderProcess.PID &&
			process.StartedAt == holderProcess.StartedAt && process.Executable == holderProcess.Executable {
			return claimedHandoffDeliveryObservation(observation, result.Execution.Lease)
		}
	}
	return nil, nil
}

func claimedHandoffDeliveryObservation(observation issueopscontract.IssueOpsHandoffDeliveryObservation, lease issueopscontract.WriteLease) (*issueopscontract.IssueOpsHandoffDeliveryObservation, error) {
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
	if err := ValidateHandoffDeliveryObservation(observation); err != nil {
		return nil, err
	}
	return &observation, nil
}
