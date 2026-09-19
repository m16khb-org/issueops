package issueopsapp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	auditadapter "issueops/internal/adapter/audit"
	issueopsadapter "issueops/internal/adapter/issueops"
	issueopscontract "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

const handoffDeliveryManualLineagePrefix = "manual-direct:"

func auditManualHandoffDeliveryObservation(observation issueopscontract.IssueOpsHandoffDeliveryObservation) (auditadapter.HandoffDeliveryAuditRecord, error) {
	record, err := issueopsadapter.ReadIssueOps(auditadapter.StateDir(), observation.LifecycleID)
	if err != nil {
		return auditadapter.HandoffDeliveryAuditRecord{}, err
	}
	if err := validateManualHandoffDeliveryObservation(record, observation); err != nil {
		return auditadapter.HandoffDeliveryAuditRecord{}, err
	}
	return auditadapter.AuditHandoffDeliveryObservation(observation)
}

func validateManualHandoffDeliveryObservation(record issueopscontract.IssueOpsRecord, observation issueopscontract.IssueOpsHandoffDeliveryObservation) error {
	if record.Execution == nil || record.Execution.Mode != issueopscontract.ExecutionModeDirect || record.Execution.Lease.Status != issueopscontract.LeaseStatusReleased ||
		record.Execution.Lease.Generation != observation.SourceGeneration || record.ID != observation.LifecycleID {
		return fmt.Errorf("manual handoff delivery observation requires the exact released direct execution generation")
	}
	if !strings.HasPrefix(observation.AttemptID, handoffDeliveryManualLineagePrefix+record.ID+":") ||
		!strings.HasPrefix(observation.LineageID, handoffDeliveryManualLineagePrefix) {
		return fmt.Errorf("manual handoff delivery observation requires an isolated manual-direct namespace")
	}
	if observation.Launcher.Name != issueopscontract.IssueOpsHandoffDeliveryLauncherOrca && observation.Launcher.Name != issueopscontract.IssueOpsHandoffDeliveryLauncherHerdr {
		return fmt.Errorf("manual handoff delivery observation launcher must be Orca or Herdr")
	}
	if observation.OwnerActor != nil || observation.OwnerClaimed.Status != issueopscontract.IssueOpsHandoffDeliveryStateNotObserved ||
		observation.OwnerClaim.Claimed || observation.OwnerClaim.Generation != 0 || observation.OwnerClaim.ClaimedAt != "" ||
		observation.OwnerClaim.Actor.Host != "" || observation.OwnerClaim.Actor.SessionID != "" || observation.OwnerClaim.Actor.AgentID != "" ||
		observation.OwnerClaim.Actor.SessionProcess != nil || len(observation.OwnerClaim.Actor.ProcessAncestry) != 0 {
		return fmt.Errorf("manual handoff delivery observation cannot produce owner claim evidence")
	}
	return nil
}

type handoffDeliveryProvisioner struct {
	stateRoot string
	next      port.ExecutionOrcaProvisioner
	now       func() time.Time
}

func newHandoffDeliveryProvisioner(stateRoot string, next port.ExecutionOrcaProvisioner, now func() time.Time) port.ExecutionOrcaProvisioner {
	if next == nil {
		return nil
	}
	if now == nil {
		now = time.Now
	}
	return &handoffDeliveryProvisioner{stateRoot: stateRoot, next: next, now: now}
}

func (provisioner *handoffDeliveryProvisioner) Probe(ctx context.Context, request port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaProbeResult, error) {
	return provisioner.next.Probe(ctx, request)
}

func (provisioner *handoffDeliveryProvisioner) InspectIntent(ctx context.Context, request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	if !isHandoffDeliveryRequest(request) {
		return provisioner.next.InspectIntent(ctx, request)
	}
	identity, err := provisioner.deliveryIdentity(ctx, request)
	if err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	recovered, handled, err := inspectHandoffDeliveryRecovery(ctx, provisioner.stateRoot, request, identity, provisioner.next)
	if err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	if handled {
		return recovered, nil
	}
	return provisioner.next.InspectIntent(ctx, request)
}

func (provisioner *handoffDeliveryProvisioner) InvokeIntent(ctx context.Context, request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
	if !isHandoffDeliveryRequest(request) {
		return provisioner.next.InvokeIntent(ctx, request)
	}
	identity, err := provisioner.deliveryIdentity(ctx, request)
	if err != nil {
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	request, err = recoverHandoffDeliveryRequest(provisioner.stateRoot, request, identity)
	if err != nil {
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	if observed, ok := provisioner.next.(port.ExecutionOrcaObservedInvoker); ok {
		receipt, err := observed.InvokeIntentObserved(ctx, request, func(event port.ExecutionOrcaCallObservation) error {
			switch event.Phase {
			case port.ExecutionOrcaCallStaged:
				return observeHandoffDeliveryStaged(provisioner.stateRoot, request, identity, event.CallKind, event.Receipt, provisioner.now)
			case port.ExecutionOrcaCallCompleted:
				return observeHandoffDeliveryCompleted(provisioner.stateRoot, request, identity, event.CallKind, event.Receipt, provisioner.now)
			default:
				return fmt.Errorf("unsupported Orca delivery call observation phase %q", event.Phase)
			}
		})
		if err != nil {
			if observeErr := observeHandoffDeliveryFailure(provisioner.stateRoot, request, identity, err, provisioner.now); observeErr != nil {
				err = errors.Join(err, observeErr)
			}
			return port.ExecutionOrcaIntentReceipt{}, err
		}
		return receipt, nil
	}
	if err := observeHandoffDeliveryBefore(provisioner.stateRoot, request, identity, provisioner.now); err != nil {
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	receipt, err := provisioner.next.InvokeIntent(ctx, request)
	if err != nil {
		if observeErr := observeHandoffDeliveryFailure(provisioner.stateRoot, request, identity, err, provisioner.now); observeErr != nil {
			err = errors.Join(err, observeErr)
		}
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	if err := observeHandoffDeliveryAfter(provisioner.stateRoot, request, identity, receipt, provisioner.now); err != nil {
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	return receipt, nil
}

func (provisioner *handoffDeliveryProvisioner) deliveryIdentity(ctx context.Context, request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaDeliveryIdentity, error) {
	observer, ok := provisioner.next.(port.ExecutionOrcaDeliveryObserver)
	if !ok {
		return port.ExecutionOrcaDeliveryIdentity{}, fmt.Errorf("Orca delivery identity observer is unavailable")
	}
	return observer.InspectDeliveryIdentity(ctx, request)
}

func isHandoffDeliveryRequest(request port.ExecutionOrcaIntentRequest) bool {
	return request.Stage == port.ExecutionOrcaIntentDispatch && request.Launch != nil
}

func observeHandoffDeliveryBefore(stateRoot string, request port.ExecutionOrcaIntentRequest, identity port.ExecutionOrcaDeliveryIdentity, now func() time.Time) error {
	return observeHandoffDeliveryStaged(stateRoot, request, identity, handoffDeliveryCallKind(request, nil), port.ExecutionOrcaIntentReceipt{}, now)
}

func observeHandoffDeliveryStaged(stateRoot string, request port.ExecutionOrcaIntentRequest, identity port.ExecutionOrcaDeliveryIdentity, callKind string, receipt port.ExecutionOrcaIntentReceipt, now func() time.Time) error {
	eventNow := handoffDeliveryEventClock(now)
	observation, err := handoffDeliveryObservation(stateRoot, request, identity, receipt, "", callKind, eventNow)
	if err != nil {
		return err
	}
	observation.CallStaged = handoffDeliveryObserved(eventNow, issueopscontract.IssueOpsHandoffDeliveryEvidenceExternalCallStaged)
	_, err = auditadapter.AuditHandoffDeliveryObservationAt(stateRoot, observation)
	return err
}

func observeHandoffDeliveryAfter(stateRoot string, request port.ExecutionOrcaIntentRequest, identity port.ExecutionOrcaDeliveryIdentity, receipt port.ExecutionOrcaIntentReceipt, now func() time.Time) error {
	if receipt.RequestID != "" {
		if err := observeHandoffDeliveryCompleted(stateRoot, request, identity, "dispatch", receipt, now); err != nil {
			return err
		}
	}
	if receipt.PromptReceipt == nil {
		return nil
	}
	return observeHandoffDeliveryCompleted(stateRoot, request, identity, "prompt", receipt, now)
}

func observeHandoffDeliveryCompleted(stateRoot string, request port.ExecutionOrcaIntentRequest, identity port.ExecutionOrcaDeliveryIdentity, callKind string, receipt port.ExecutionOrcaIntentReceipt, now func() time.Time) error {
	eventNow := handoffDeliveryEventClock(now)
	durableID := strings.TrimSpace(receipt.RequestID)
	if callKind == "prompt" && receipt.PromptReceipt != nil {
		durableID = strings.TrimSpace(receipt.PromptReceipt.RequestID)
	}
	observation, err := handoffDeliveryObservation(stateRoot, request, identity, receipt, durableID, callKind, eventNow)
	if err != nil {
		return err
	}
	if callKind == "dispatch" {
		if request.Probe.Host == "omo" {
			observation.Ambiguous = handoffDeliveryObserved(eventNow, issueopscontract.IssueOpsHandoffDeliveryEvidenceOrcaDispatchReceipt)
		} else {
			observation.InputAccepted = handoffDeliveryObserved(eventNow, issueopscontract.IssueOpsHandoffDeliveryEvidenceOrcaDispatch)
		}
	} else {
		if receipt.PromptReceipt == nil {
			return fmt.Errorf("Omo prompt completion is missing its durable receipt")
		}
		observation.Target.ProcessIncarnation = strings.TrimSpace(receipt.PromptReceipt.ProcessIncarnation)
		observation.InputAccepted = handoffDeliveryObserved(eventNow, issueopscontract.IssueOpsHandoffDeliveryEvidenceOmoSendAccepted)
		if containsHandoffDeliveryString(receipt.PromptReceipt.Stages, "turn_started") {
			observation.NativeTurnObserved = handoffDeliveryObserved(eventNow, issueopscontract.IssueOpsHandoffDeliveryEvidenceNativeReceipt)
		}
	}
	_, err = auditadapter.AuditHandoffDeliveryObservationAt(stateRoot, observation)
	return err
}

func observeHandoffDeliveryFailure(stateRoot string, request port.ExecutionOrcaIntentRequest, identity port.ExecutionOrcaDeliveryIdentity, err error, now func() time.Time) error {
	callKind := handoffDeliveryCallKind(request, err)
	eventNow := handoffDeliveryEventClock(now)
	observation, observeErr := handoffDeliveryObservation(stateRoot, request, identity, port.ExecutionOrcaIntentReceipt{}, "", callKind, eventNow)
	if observeErr != nil {
		return observeErr
	}
	if typed, ok := errors.AsType[*port.OrcaError](err); ok {
		if !typed.Invoked || typed.Code == "delivery_observation_failed" {
			return nil
		}
		observation.Request.DurableID = strings.TrimSpace(typed.OrchestrationRequestID)
		if typed.CallPhase == "terminal_send" {
			if strings.TrimSpace(typed.DispatchRequestID) != "" {
				dispatchNow := handoffDeliveryEventClock(now)
				dispatchObservation, dispatchErr := handoffDeliveryObservation(stateRoot, request, identity, port.ExecutionOrcaIntentReceipt{}, strings.TrimSpace(typed.DispatchRequestID), "dispatch", dispatchNow)
				if dispatchErr != nil {
					return dispatchErr
				}
				dispatchObservation.Ambiguous = handoffDeliveryObserved(dispatchNow, issueopscontract.IssueOpsHandoffDeliveryEvidenceOrcaDispatchReceipt)
				if _, dispatchErr := auditadapter.AuditHandoffDeliveryObservationAt(stateRoot, dispatchObservation); dispatchErr != nil {
					return dispatchErr
				}
			}
			observation.Ambiguous = handoffDeliveryObserved(eventNow, issueopscontract.IssueOpsHandoffDeliveryEvidenceOmoSendResponseLost)
		} else {
			observation.Ambiguous = handoffDeliveryObserved(eventNow, issueopscontract.IssueOpsHandoffDeliveryEvidenceAcceptedResponseLost)
		}
	} else {
		observation.Ambiguous = handoffDeliveryObserved(eventNow, issueopscontract.IssueOpsHandoffDeliveryEvidenceAcceptedResponseLost)
	}
	_, observeErr = auditadapter.AuditHandoffDeliveryObservationAt(stateRoot, observation)
	return observeErr
}

func inspectHandoffDeliveryRecovery(ctx context.Context, stateRoot string, request port.ExecutionOrcaIntentRequest, identity port.ExecutionOrcaDeliveryIdentity, provisioner port.ExecutionOrcaProvisioner) (port.ExecutionOrcaIntentInventory, bool, error) {
	dispatchObservation, dispatchFound, err := foldedHandoffDeliveryObservation(stateRoot, request, identity, "dispatch")
	if err != nil {
		return port.ExecutionOrcaIntentInventory{}, false, err
	}
	promptObservation, promptFound, err := foldedHandoffDeliveryObservation(stateRoot, request, identity, "prompt")
	if err != nil {
		return port.ExecutionOrcaIntentInventory{}, false, err
	}
	observer, ok := provisioner.(port.ExecutionOrcaDeliveryObserver)
	if !ok {
		return port.ExecutionOrcaIntentInventory{}, false, fmt.Errorf("Orca delivery request observer is unavailable")
	}
	dispatchRequestID := firstNonEmptyIssueOpsApp(request.RetryRequestID, dispatchObservation.Request.DurableID)
	if strings.TrimSpace(request.RetryRequestID) != "" && !dispatchFound {
		return port.ExecutionOrcaIntentInventory{}, false, fmt.Errorf("Orca dispatch retry request has no matching delivery observation")
	}
	dispatchStatus := ""
	if dispatchRequestID != "" {
		dispatchStatus, err = observeHandoffDeliveryRequest(ctx, observer, dispatchRequestID, "orchestration.dispatch", identity.RuntimeID)
		if err != nil {
			return port.ExecutionOrcaIntentInventory{}, false, err
		}
	}
	if request.Probe.Host != "omo" {
		if dispatchStatus == "completed" || dispatchStatus == "pending" {
			dispatchReceipt, exists, inspectErr := observer.InspectDeliveryDispatch(ctx, request)
			if inspectErr != nil {
				return port.ExecutionOrcaIntentInventory{}, false, inspectErr
			}
			if exists {
				dispatchReceipt.RequestID = dispatchRequestID
				return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{dispatchReceipt}}, true, nil
			}
			return port.ExecutionOrcaIntentInventory{ExactReplay: true}, true, nil
		}
		return port.ExecutionOrcaIntentInventory{}, false, nil
	}

	dispatchReceipt, dispatchExists, err := observer.InspectDeliveryDispatch(ctx, request)
	if err != nil {
		return port.ExecutionOrcaIntentInventory{}, false, err
	}
	if dispatchExists {
		dispatchReceipt.RequestID = dispatchRequestID
	}
	if promptFound && (promptObservation.InputAccepted.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved || promptObservation.OwnerClaimed.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved) {
		if !dispatchExists || strings.TrimSpace(promptObservation.Request.DurableID) == "" || strings.TrimSpace(promptObservation.Target.ProcessIncarnation) == "" {
			return port.ExecutionOrcaIntentInventory{}, false, fmt.Errorf("Omo prompt delivery recovery evidence is incomplete")
		}
		stages := []string{"input_accepted"}
		if promptObservation.NativeTurnObserved.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved {
			stages = append(stages, "turn_started")
		}
		dispatchReceipt.PromptReceipt = &port.OrcaPromptReceipt{
			RequestID: promptObservation.Request.DurableID, Stages: stages, Provider: "omo",
			ProcessIncarnation: promptObservation.Target.ProcessIncarnation, Generation: request.SourceGeneration,
		}
		return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{dispatchReceipt}}, true, nil
	}
	promptRequestID := firstNonEmptyIssueOpsApp(request.PromptRetryRequestID, promptObservation.Request.DurableID)
	if strings.TrimSpace(request.PromptRetryRequestID) != "" && !promptFound {
		return port.ExecutionOrcaIntentInventory{}, false, fmt.Errorf("Orca prompt retry request has no matching delivery observation")
	}
	if promptRequestID != "" {
		// Orca request-show is scoped to orchestration mutations. A terminal
		// prompt UUID is recovered only by replaying the exact terminal send;
		// Orca binds that replay to the prompt payload and process incarnation.
		return port.ExecutionOrcaIntentInventory{ExactReplay: true}, true, nil
	}
	if dispatchFound && dispatchRequestID != "" && (dispatchStatus == "completed" || dispatchStatus == "pending") {
		return port.ExecutionOrcaIntentInventory{ExactReplay: true}, true, nil
	}
	return port.ExecutionOrcaIntentInventory{}, false, nil
}

func recoverHandoffDeliveryRequest(stateRoot string, request port.ExecutionOrcaIntentRequest, identity port.ExecutionOrcaDeliveryIdentity) (port.ExecutionOrcaIntentRequest, error) {
	dispatch, dispatchFound, err := foldedHandoffDeliveryObservation(stateRoot, request, identity, "dispatch")
	if err != nil {
		return request, err
	}
	prompt, promptFound, err := foldedHandoffDeliveryObservation(stateRoot, request, identity, "prompt")
	if err != nil {
		return request, err
	}
	if request.RetryRequestID == "" && dispatchFound {
		request.RetryRequestID = strings.TrimSpace(dispatch.Request.DurableID)
	}
	if request.PromptRetryRequestID == "" && promptFound {
		request.PromptRetryRequestID = strings.TrimSpace(prompt.Request.DurableID)
	}
	if promptFound {
		request.ExpectedPromptProcessIncarnation = strings.TrimSpace(prompt.Target.ProcessIncarnation)
	}
	return request, nil
}

func foldedHandoffDeliveryObservation(stateRoot string, request port.ExecutionOrcaIntentRequest, identity port.ExecutionOrcaDeliveryIdentity, callKind string) (issueopscontract.IssueOpsHandoffDeliveryObservation, bool, error) {
	probe := handoffDeliveryObservationProbe(request, identity, callKind)
	folded, decisions, err := auditadapter.FoldHandoffDeliveryAuditObservationsForAt(stateRoot, probe.LifecycleID, probe.LineageID)
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
	key := issueopsdomain.HandoffDeliveryFoldKey(probe)
	if key == "" {
		return issueopscontract.IssueOpsHandoffDeliveryObservation{}, false, nil
	}
	observation, ok := folded[key]
	if !ok {
		return issueopscontract.IssueOpsHandoffDeliveryObservation{}, false, nil
	}
	if observation.PromptSHA256 != probe.PromptSHA256 || observation.MaterialSHA256 != probe.MaterialSHA256 || observation.SourceGeneration != probe.SourceGeneration ||
		observation.Launcher != probe.Launcher || observation.ExpectedOwnerHost != probe.ExpectedOwnerHost ||
		observation.Target.TerminalID != "" && observation.Target.TerminalID != identity.TerminalPTYID ||
		observation.Target.PaneID != "" && observation.Target.PaneID != identity.TerminalHandle {
		return issueopscontract.IssueOpsHandoffDeliveryObservation{}, false, fmt.Errorf("handoff delivery recovery evidence conflicts with current request identity")
	}
	return observation, true, nil
}

func observeHandoffDeliveryRequest(ctx context.Context, observer port.ExecutionOrcaDeliveryObserver, requestID, method, runtimeID string) (string, error) {
	observed, err := observer.ObserveRequest(ctx, requestID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(observed.RequestID) != strings.TrimSpace(requestID) || strings.TrimSpace(observed.Method) != method || strings.TrimSpace(observed.RuntimeID) != strings.TrimSpace(runtimeID) {
		return "", fmt.Errorf("Orca request observation identity mismatch")
	}
	switch strings.TrimSpace(observed.Status) {
	case "completed", "pending":
		return strings.TrimSpace(observed.Status), nil
	case "absent":
		return "", fmt.Errorf("Orca durable request is absent; recovery remains ambiguous")
	default:
		return "", fmt.Errorf("Orca request observation state is invalid")
	}
}

func handoffDeliveryObservation(stateRoot string, request port.ExecutionOrcaIntentRequest, identity port.ExecutionOrcaDeliveryIdentity, receipt port.ExecutionOrcaIntentReceipt, durableID, callKind string, now func() time.Time) (issueopscontract.IssueOpsHandoffDeliveryObservation, error) {
	timestamp := now().UTC().Format(time.RFC3339Nano)
	createdAt := timestamp
	probe := handoffDeliveryObservationProbe(request, identity, callKind)
	folded, _, err := auditadapter.FoldHandoffDeliveryAuditObservationsForAt(stateRoot, probe.LifecycleID, probe.LineageID)
	if err != nil {
		return issueopscontract.IssueOpsHandoffDeliveryObservation{}, err
	}
	if current, ok := folded[issueopsdomain.HandoffDeliveryFoldKey(probe)]; ok {
		createdAt = current.CreatedAt
	}
	return issueopscontract.IssueOpsHandoffDeliveryObservation{
		SchemaVersion:     issueopscontract.IssueOpsHandoffDeliverySchemaVersion,
		AttemptID:         strings.TrimSpace(request.OperationID) + ":" + string(request.Stage) + ":" + callKind,
		LineageID:         handoffDeliveryLineageID(request, callKind),
		LifecycleID:       strings.TrimSpace(request.Workspace.LifecycleID),
		PromptSHA256:      strings.TrimSpace(request.Launch.PromptSHA256),
		MaterialSHA256:    strings.TrimSpace(request.Launch.ContextPacketSHA256),
		Request:           issueopscontract.IssueOpsHandoffDeliveryRequest{DurableID: strings.TrimSpace(durableID)},
		ExpectedOwnerHost: strings.TrimSpace(request.Probe.Host),
		Launcher:          handoffDeliveryLauncher(identity),
		Target: issueopscontract.IssueOpsHandoffDeliveryTarget{
			TerminalID: firstNonEmptyIssueOpsApp(receipt.TerminalPTYID, identity.TerminalPTYID, request.TerminalPTYID),
			PaneID:     firstNonEmptyIssueOpsApp(receipt.TerminalHandle, identity.TerminalHandle),
		},
		SourceGeneration:   request.SourceGeneration,
		CreatedAt:          createdAt,
		UpdatedAt:          timestamp,
		Receipt:            issueopscontract.IssueOpsHandoffDeliveryReceipt{},
		InputAccepted:      issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateNotObserved},
		NativeTurnObserved: issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateNotObserved},
		OwnerClaimed:       issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateNotObserved},
		Ambiguous:          issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateNotObserved},
	}, nil
}

func handoffDeliveryObservationProbe(request port.ExecutionOrcaIntentRequest, identity port.ExecutionOrcaDeliveryIdentity, callKind string) issueopscontract.IssueOpsHandoffDeliveryObservation {
	return issueopscontract.IssueOpsHandoffDeliveryObservation{
		LineageID:         handoffDeliveryLineageID(request, callKind),
		LifecycleID:       strings.TrimSpace(request.Workspace.LifecycleID),
		PromptSHA256:      strings.TrimSpace(request.Launch.PromptSHA256),
		MaterialSHA256:    strings.TrimSpace(request.Launch.ContextPacketSHA256),
		SourceGeneration:  request.SourceGeneration,
		ExpectedOwnerHost: strings.TrimSpace(request.Probe.Host),
		Launcher:          handoffDeliveryLauncher(identity),
	}
}

func handoffDeliveryLauncher(identity port.ExecutionOrcaDeliveryIdentity) issueopscontract.IssueOpsHandoffDeliveryLauncher {
	return issueopscontract.IssueOpsHandoffDeliveryLauncher{
		Name: issueopscontract.IssueOpsHandoffDeliveryLauncherOrca, Version: strings.TrimSpace(identity.Version), Path: strings.TrimSpace(identity.LauncherPath),
		RuntimeID: strings.TrimSpace(identity.RuntimeID), MachineID: strings.TrimSpace(identity.MachineID), ServerID: strings.TrimSpace(identity.TargetIdentity),
	}
}

func handoffDeliveryLineageID(request port.ExecutionOrcaIntentRequest, callKind string) string {
	return strings.Join([]string{
		"generation", strconv.FormatInt(int64(request.SourceGeneration), 10),
		"prompt", strings.TrimSpace(request.Launch.PromptSHA256),
		"material", strings.TrimSpace(request.Launch.ContextPacketSHA256),
		"call", strings.TrimSpace(callKind),
	}, ":")
}

func handoffDeliveryCallKind(request port.ExecutionOrcaIntentRequest, err error) string {
	if typed, ok := errors.AsType[*port.OrcaError](err); ok && typed.CallPhase == "terminal_send" {
		return "prompt"
	}
	if request.Probe.Host == "omo" && request.Stage == port.ExecutionOrcaIntentDispatch {
		return "dispatch"
	}
	return string(request.Stage)
}

func handoffDeliveryObserved(now func() time.Time, evidence string) issueopscontract.IssueOpsHandoffDeliveryState {
	return issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateObserved, ObservedAt: now().UTC().Format(time.RFC3339Nano), Evidence: evidence}
}

func handoffDeliveryEventClock(now func() time.Time) func() time.Time {
	timestamp := now()
	return func() time.Time { return timestamp }
}

func containsHandoffDeliveryString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func firstNonEmptyIssueOpsApp(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
