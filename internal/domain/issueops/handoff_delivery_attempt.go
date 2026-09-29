package issueops

import (
	"fmt"
	"strconv"
	"strings"

	issueopscontract "issueops/internal/contract/issueops"
)

type DeliveryAttempt struct {
	OperationID, Stage, LifecycleID, Host, PromptSHA256, MaterialSHA256 string
	SourceGeneration                                                    uint64
	RetryRequestID, PromptRetryRequestID, TerminalID                    string
	HasLaunch                                                           bool
}
type DeliveryFailure struct {
	Code, CallPhase, OrchestrationRequestID, DispatchRequestID string
	Invoked                                                    bool
}
type DeliveryReceiptIdentity struct{ RequestID, TaskID, DispatchID, TerminalID, TerminalHandle string }

func (a DeliveryAttempt) AttemptID(callKind string) string {
	return strings.TrimSpace(a.OperationID) + ":" + a.Stage + ":" + callKind
}
func (a DeliveryAttempt) LineageID(callKind string) string {
	return strings.Join([]string{"generation", strconv.FormatInt(int64(a.SourceGeneration), 10), "prompt", strings.TrimSpace(a.PromptSHA256), "material", strings.TrimSpace(a.MaterialSHA256), "call", strings.TrimSpace(callKind)}, ":")
}
func (a DeliveryAttempt) Probe(launcher issueopscontract.IssueOpsHandoffDeliveryLauncher, callKind string) issueopscontract.IssueOpsHandoffDeliveryObservation {
	return issueopscontract.IssueOpsHandoffDeliveryObservation{LineageID: a.LineageID(callKind), LifecycleID: strings.TrimSpace(a.LifecycleID), PromptSHA256: strings.TrimSpace(a.PromptSHA256), MaterialSHA256: strings.TrimSpace(a.MaterialSHA256), SourceGeneration: a.SourceGeneration, ExpectedOwnerHost: strings.TrimSpace(a.Host), Launcher: launcher}
}
func (a DeliveryAttempt) IsDelivery() bool { return a.Stage == "dispatch" && a.HasLaunch }
func (a DeliveryAttempt) CallKind(failure *DeliveryFailure) string {
	if failure != nil && failure.CallPhase == "terminal_send" {
		return "prompt"
	}
	if a.Host == "omo" && a.Stage == "dispatch" {
		return "dispatch"
	}
	return a.Stage
}

func ValidateDeliveryRetryRequestIDs(request DeliveryAttempt) error {
	if err := issueopscontract.ValidateOrcaRetryRequestID(request.RetryRequestID); err != nil {
		return fmt.Errorf("Orca dispatch retry request ID is invalid")
	}
	if err := issueopscontract.ValidateOrcaRetryRequestID(request.PromptRetryRequestID); err != nil {
		return fmt.Errorf("Orca prompt retry request ID is invalid")
	}
	return nil
}

func DeliveryDurableRequestID(requestID, observedID string, found bool, callKind string) (string, error) {
	requestID = strings.TrimSpace(requestID)
	observedID = strings.TrimSpace(observedID)
	if err := issueopscontract.ValidateOrcaRetryRequestID(requestID); err != nil {
		return "", fmt.Errorf("Orca %s retry request ID is invalid", callKind)
	}
	if observedID != "" && issueopscontract.ValidateOrcaRequestID(observedID) != nil {
		return "", fmt.Errorf("Orca %s delivery observation has an invalid durable request ID", callKind)
	}
	if requestID != "" {
		if !found || observedID == "" || requestID != observedID {
			return "", fmt.Errorf("Orca %s retry request has no exact delivery observation", callKind)
		}
		return requestID, nil
	}
	return observedID, nil
}

func DeliveryResponseIdentityRejected(request DeliveryAttempt, typed *DeliveryFailure) bool {
	if typed == nil {
		return false
	}
	observed := strings.TrimSpace(typed.OrchestrationRequestID)
	sealed := strings.TrimSpace(request.RetryRequestID)
	if typed.CallPhase == "terminal_send" {
		sealed = strings.TrimSpace(request.PromptRetryRequestID)
		dispatchObserved := strings.TrimSpace(typed.DispatchRequestID)
		dispatchSealed := strings.TrimSpace(request.RetryRequestID)
		if dispatchObserved != "" && (issueopscontract.ValidateOrcaRequestID(dispatchObserved) != nil || dispatchSealed != "" && dispatchObserved != dispatchSealed) {
			return true
		}
	}
	identityError := typed.Code == "dispatch_request_identity_mismatch" ||
		typed.Code == "terminal_prompt_request_identity_mismatch" ||
		typed.Code == "terminal_prompt_receipt_invalid"
	if observed == "" {
		return identityError
	}
	return issueopscontract.ValidateOrcaRequestID(observed) != nil || sealed != "" && observed != sealed
}

func DeliveryAcceptedResponseID(sealed, observed string) string {
	sealed = strings.TrimSpace(sealed)
	observed = strings.TrimSpace(observed)
	if issueopscontract.ValidateOrcaRequestID(observed) != nil || sealed != "" && observed != sealed {
		return ""
	}
	return observed
}

func RejectStagedDeliveryWithoutResponse(request DeliveryAttempt, observations []issueopscontract.IssueOpsHandoffDeliveryObservation) error {
	for _, callKind := range []string{"dispatch", "prompt"} {
		attemptID := strings.TrimSpace(request.OperationID) + ":" + string(request.Stage) + ":" + callKind
		lineageID := request.LineageID(callKind)
		var latest *issueopscontract.IssueOpsHandoffDeliveryObservation
		for index := range observations {
			observation := &observations[index]
			if observation.AttemptID == attemptID && observation.LineageID == lineageID &&
				observation.LifecycleID == strings.TrimSpace(request.LifecycleID) &&
				observation.PromptSHA256 == strings.TrimSpace(request.PromptSHA256) &&
				observation.MaterialSHA256 == strings.TrimSpace(request.MaterialSHA256) &&
				observation.SourceGeneration == request.SourceGeneration &&
				observation.ExpectedOwnerHost == strings.TrimSpace(request.Host) {
				latest = observation
			}
		}
		if latest != nil && latest.CallStaged.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved &&
			strings.TrimSpace(latest.Request.DurableID) == "" &&
			latest.InputAccepted.Status != issueopscontract.IssueOpsHandoffDeliveryStateObserved &&
			latest.Ambiguous.Status != issueopscontract.IssueOpsHandoffDeliveryStateObserved {
			return fmt.Errorf("Orca %s delivery is ambiguous after a staged call without an accepted response identity", callKind)
		}
	}
	return nil
}

func ValidateDeliveryDispatchReceipt(receipt DeliveryReceiptIdentity, taskID, terminalID, terminalHandle, requestID string) error {
	if strings.TrimSpace(receipt.RequestID) != strings.TrimSpace(requestID) ||
		strings.TrimSpace(receipt.TaskID) != strings.TrimSpace(taskID) ||
		strings.TrimSpace(receipt.DispatchID) == "" ||
		strings.TrimSpace(receipt.TerminalID) != strings.TrimSpace(terminalID) ||
		strings.TrimSpace(receipt.TerminalHandle) != strings.TrimSpace(terminalHandle) {
		return fmt.Errorf("Orca dispatch candidate does not match the durable request and delivery terminal identity")
	}
	return nil
}

func RecoverDeliveryObservation(probe issueopscontract.IssueOpsHandoffDeliveryObservation, terminalID string, observations []issueopscontract.IssueOpsHandoffDeliveryObservation, err error) (issueopscontract.IssueOpsHandoffDeliveryObservation, bool, error) {
	folded, decisions := FoldHandoffDeliveryObservations(observations)
	for _, decision := range decisions {
		if !decision.Accepted {
			if err != nil {
				return issueopscontract.IssueOpsHandoffDeliveryObservation{}, false, fmt.Errorf("handoff delivery recovery evidence rejected: %w", err)
			}
			return issueopscontract.IssueOpsHandoffDeliveryObservation{}, false, fmt.Errorf("handoff delivery recovery evidence rejected: %s", strings.Join(decision.RejectReasons, "; "))
		}
	}
	if err != nil {
		return issueopscontract.IssueOpsHandoffDeliveryObservation{}, false, err
	}
	key := HandoffDeliveryFoldKey(probe)
	if key == "" {
		return issueopscontract.IssueOpsHandoffDeliveryObservation{}, false, nil
	}
	observation, ok := folded[key]
	if !ok {
		return issueopscontract.IssueOpsHandoffDeliveryObservation{}, false, nil
	}
	if observation.PromptSHA256 != probe.PromptSHA256 || observation.MaterialSHA256 != probe.MaterialSHA256 || observation.SourceGeneration != probe.SourceGeneration ||
		observation.Launcher != probe.Launcher || observation.ExpectedOwnerHost != probe.ExpectedOwnerHost ||
		observation.Target.TerminalID != "" && observation.Target.TerminalID != terminalID {
		return issueopscontract.IssueOpsHandoffDeliveryObservation{}, false, fmt.Errorf("handoff delivery recovery evidence conflicts with current request identity")
	}
	return observation, true, nil
}

func DeliveryRequestStatus(requestID, method, runtimeID, observedID, observedMethod, observedRuntime, status string) (string, error) {
	if strings.TrimSpace(observedID) != strings.TrimSpace(requestID) || strings.TrimSpace(observedMethod) != method || strings.TrimSpace(observedRuntime) != strings.TrimSpace(runtimeID) {
		return "", fmt.Errorf("Orca request observation identity mismatch")
	}
	switch strings.TrimSpace(status) {
	case "completed", "pending":
		return strings.TrimSpace(status), nil
	case "absent":
		return "", fmt.Errorf("Orca durable request is absent; recovery remains ambiguous")
	default:
		return "", fmt.Errorf("Orca request observation state is invalid")
	}
}
