package handoffdelivery

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

type Provisioner struct {
	Audit Audit
	Next  port.ExecutionOrcaProvisioner
	Now   func() time.Time
}

func (provisioner *Provisioner) Probe(ctx context.Context, request port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaProbeResult, error) {
	return provisioner.Next.Probe(ctx, request)
}

func (provisioner *Provisioner) InspectIntent(ctx context.Context, request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	if !deliveryAttempt(request).IsDelivery() {
		return provisioner.Next.InspectIntent(ctx, request)
	}
	if err := issueopsdomain.ValidateDeliveryRetryRequestIDs(deliveryAttempt(request)); err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	if err := rejectStagedHandoffDeliveryWithoutResponse(provisioner.Audit, request); err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	identity, err := provisioner.deliveryIdentity(ctx, request)
	if err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	recovered, handled, err := inspectHandoffDeliveryRecovery(ctx, provisioner.Audit, request, identity, provisioner.Next)
	if err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	if handled {
		return recovered, nil
	}
	return provisioner.Next.InspectIntent(ctx, request)
}

func (provisioner *Provisioner) InvokeIntent(ctx context.Context, request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
	if !deliveryAttempt(request).IsDelivery() {
		return provisioner.Next.InvokeIntent(ctx, request)
	}
	if err := issueopsdomain.ValidateDeliveryRetryRequestIDs(deliveryAttempt(request)); err != nil {
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	if err := rejectStagedHandoffDeliveryWithoutResponse(provisioner.Audit, request); err != nil {
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	identity, err := provisioner.deliveryIdentity(ctx, request)
	if err != nil {
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	request, err = recoverHandoffDeliveryRequest(provisioner.Audit, request, identity)
	if err != nil {
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	if observed, ok := provisioner.Next.(port.ExecutionOrcaObservedInvoker); ok {
		receipt, err := observed.InvokeIntentObserved(ctx, request, func(event port.ExecutionOrcaCallObservation) error {
			switch event.Phase {
			case port.ExecutionOrcaCallStaged:
				return ObserveStaged(provisioner.Audit, request, identity, event.CallKind, event.Receipt, provisioner.Now)
			case port.ExecutionOrcaCallCompleted:
				return ObserveCompleted(provisioner.Audit, request, identity, event.CallKind, event.Receipt, provisioner.Now)
			default:
				return fmt.Errorf("unsupported Orca delivery call observation phase %q", event.Phase)
			}
		})
		if err != nil {
			if observeErr := ObserveFailure(provisioner.Audit, request, identity, err, provisioner.Now); observeErr != nil {
				err = errors.Join(err, observeErr)
			}
			return port.ExecutionOrcaIntentReceipt{}, err
		}
		return receipt, nil
	}
	if err := observeHandoffDeliveryBefore(provisioner.Audit, request, identity, provisioner.Now); err != nil {
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	receipt, err := provisioner.Next.InvokeIntent(ctx, request)
	if err != nil {
		if observeErr := ObserveFailure(provisioner.Audit, request, identity, err, provisioner.Now); observeErr != nil {
			err = errors.Join(err, observeErr)
		}
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	if err := observeHandoffDeliveryAfter(provisioner.Audit, request, identity, receipt, provisioner.Now); err != nil {
		return port.ExecutionOrcaIntentReceipt{}, err
	}
	return receipt, nil
}

func rejectStagedHandoffDeliveryWithoutResponse(audit Audit, request port.ExecutionOrcaIntentRequest) error {
	observations, err := audit.Read()
	if err != nil {
		return err
	}
	return issueopsdomain.RejectStagedDeliveryWithoutResponse(deliveryAttempt(request), observations)
}

func (provisioner *Provisioner) deliveryIdentity(ctx context.Context, request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaDeliveryIdentity, error) {
	observer, ok := provisioner.Next.(port.ExecutionOrcaDeliveryObserver)
	if !ok {
		return port.ExecutionOrcaDeliveryIdentity{}, fmt.Errorf("Orca delivery identity observer is unavailable")
	}
	return observer.InspectDeliveryIdentity(ctx, request)
}

func observeHandoffDeliveryBefore(audit Audit, request port.ExecutionOrcaIntentRequest, identity port.ExecutionOrcaDeliveryIdentity, now func() time.Time) error {
	return ObserveStaged(audit, request, identity, handoffDeliveryCallKind(request, nil), port.ExecutionOrcaIntentReceipt{}, now)
}

func ObserveStaged(audit Audit, request port.ExecutionOrcaIntentRequest, identity port.ExecutionOrcaDeliveryIdentity, callKind string, receipt port.ExecutionOrcaIntentReceipt, now func() time.Time) error {
	eventNow := handoffDeliveryEventClock(now)
	observation, err := Observation(audit, request, identity, receipt, "", callKind, eventNow)
	if err != nil {
		return err
	}
	observation.CallStaged = handoffDeliveryObserved(eventNow, issueopscontract.IssueOpsHandoffDeliveryEvidenceExternalCallStaged)
	_, err = audit.Append(observation)
	return err
}

func observeHandoffDeliveryAfter(audit Audit, request port.ExecutionOrcaIntentRequest, identity port.ExecutionOrcaDeliveryIdentity, receipt port.ExecutionOrcaIntentReceipt, now func() time.Time) error {
	if receipt.RequestID != "" {
		if err := ObserveCompleted(audit, request, identity, "dispatch", receipt, now); err != nil {
			return err
		}
	}
	if receipt.PromptReceipt == nil {
		return nil
	}
	return ObserveCompleted(audit, request, identity, "prompt", receipt, now)
}

func ObserveCompleted(audit Audit, request port.ExecutionOrcaIntentRequest, identity port.ExecutionOrcaDeliveryIdentity, callKind string, receipt port.ExecutionOrcaIntentReceipt, now func() time.Time) error {
	eventNow := handoffDeliveryEventClock(now)
	prompt := deliveryPrompt(receipt.PromptReceipt)
	durableID := issueopsdomain.DeliveryCompletionID(receipt.RequestID, callKind, prompt)
	observation, err := Observation(audit, request, identity, receipt, durableID, callKind, eventNow)
	if err != nil {
		return err
	}
	observation, err = issueopsdomain.CompleteDeliveryObservation(observation, request.Probe.Host, callKind, eventNow().UTC().Format(time.RFC3339Nano), prompt)
	if err != nil {
		return err
	}
	_, err = audit.Append(observation)
	return err
}

func ObserveFailure(audit Audit, request port.ExecutionOrcaIntentRequest, identity port.ExecutionOrcaDeliveryIdentity, err error, now func() time.Time) error {
	callKind := handoffDeliveryCallKind(request, err)
	eventNow := handoffDeliveryEventClock(now)
	observation, observeErr := Observation(audit, request, identity, port.ExecutionOrcaIntentReceipt{}, "", callKind, eventNow)
	if observeErr != nil {
		return observeErr
	}
	typed, _ := errors.AsType[*port.OrcaError](err)
	plan := issueopsdomain.DeliveryFailurePlan(deliveryAttempt(request), deliveryFailure(typed))
	if plan.Skip {
		return nil
	}
	observation.Request.DurableID = plan.DurableID
	if plan.DispatchID != "" {
		dispatchNow := handoffDeliveryEventClock(now)
		dispatchObservation, dispatchErr := Observation(audit, request, identity, port.ExecutionOrcaIntentReceipt{}, plan.DispatchID, "dispatch", dispatchNow)
		if dispatchErr != nil {
			return dispatchErr
		}
		dispatchObservation.Ambiguous = handoffDeliveryObserved(dispatchNow, issueopscontract.IssueOpsHandoffDeliveryEvidenceOrcaDispatchReceipt)
		if _, dispatchErr := audit.Append(dispatchObservation); dispatchErr != nil {
			return dispatchErr
		}
	}
	observation.Ambiguous = handoffDeliveryObserved(eventNow, plan.Evidence)
	_, observeErr = audit.Append(observation)
	return observeErr
}

func inspectHandoffDeliveryRecovery(ctx context.Context, audit Audit, request port.ExecutionOrcaIntentRequest, identity port.ExecutionOrcaDeliveryIdentity, provisioner port.ExecutionOrcaProvisioner) (port.ExecutionOrcaIntentInventory, bool, error) {
	dispatch, dispatchFound, err := foldedHandoffDeliveryObservation(audit, request, identity, "dispatch")
	if err != nil {
		return port.ExecutionOrcaIntentInventory{}, false, err
	}
	prompt, promptFound, err := foldedHandoffDeliveryObservation(audit, request, identity, "prompt")
	if err != nil {
		return port.ExecutionOrcaIntentInventory{}, false, err
	}
	observer, ok := provisioner.(port.ExecutionOrcaDeliveryObserver)
	if !ok {
		return port.ExecutionOrcaIntentInventory{}, false, fmt.Errorf("Orca delivery request observer is unavailable")
	}
	plan, err := issueopsdomain.PlanDeliveryRecovery(deliveryAttempt(request), dispatch, prompt, dispatchFound, promptFound)
	if err != nil || !plan.Active {
		return port.ExecutionOrcaIntentInventory{}, false, err
	}
	status := ""
	if plan.DispatchID != "" {
		status, err = observeHandoffDeliveryRequest(ctx, observer, plan.DispatchID, "orchestration.dispatch", identity.RuntimeID)
		if err != nil {
			return port.ExecutionOrcaIntentInventory{}, false, err
		}
	}
	inspect, err := issueopsdomain.InspectDeliveryDispatch(request.Probe.Host, dispatchFound, status)
	if err != nil || !inspect {
		return port.ExecutionOrcaIntentInventory{}, false, err
	}
	inspectRequest := request
	inspectRequest.RetryRequestID = plan.DispatchID
	receipt, exists, err := observer.InspectDeliveryDispatch(ctx, inspectRequest)
	if err != nil {
		return port.ExecutionOrcaIntentInventory{}, false, err
	}
	if exists {
		if err := issueopsdomain.ValidateDeliveryDispatchReceipt(deliveryReceiptIdentity(receipt), request.TaskID, identity.TerminalPTYID, identity.TerminalHandle, plan.DispatchID); err != nil {
			return port.ExecutionOrcaIntentInventory{}, false, err
		}
	}
	conclusion, err := issueopsdomain.FinishDeliveryRecovery(request.Probe.Host, plan, status, exists, prompt, promptFound)
	if err != nil {
		return port.ExecutionOrcaIntentInventory{}, false, err
	}
	if conclusion.Prompt != nil {
		p := conclusion.Prompt
		receipt.PromptReceipt = &port.OrcaPromptReceipt{RequestID: p.RequestID, Stages: p.Stages, Provider: "omo", ProcessIncarnation: p.ProcessIncarnation, Generation: p.Generation, BaselineWorkingSequence: p.BaselineWorkingSequence}
		if err := port.ValidateExecutionOrcaDeliveryReceipt(receipt, port.OrcaDeliveryReceiptExpectation{Host: request.Probe.Host, TaskID: request.TaskID, TerminalPTYID: identity.TerminalPTYID, TerminalHandle: identity.TerminalHandle, DispatchRequestID: plan.DispatchID, PromptRequestID: p.RequestID, PromptProcessIncarnation: p.ProcessIncarnation}); err != nil {
			return port.ExecutionOrcaIntentInventory{}, false, fmt.Errorf("Omo prompt delivery recovery evidence is incomplete: %w", err)
		}
	}
	switch conclusion.Mode {
	case issueopsdomain.DeliveryRecoveryCandidate:
		return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{receipt}}, true, nil
	case issueopsdomain.DeliveryRecoveryReplay:
		return port.ExecutionOrcaIntentInventory{ExactReplay: true}, true, nil
	default:
		return port.ExecutionOrcaIntentInventory{}, false, nil
	}
}

func recoverHandoffDeliveryRequest(audit Audit, request port.ExecutionOrcaIntentRequest, identity port.ExecutionOrcaDeliveryIdentity) (port.ExecutionOrcaIntentRequest, error) {
	dispatch, dispatchFound, err := foldedHandoffDeliveryObservation(audit, request, identity, "dispatch")
	if err != nil {
		return request, err
	}
	prompt, promptFound, err := foldedHandoffDeliveryObservation(audit, request, identity, "prompt")
	if err != nil {
		return request, err
	}
	request.RetryRequestID, err = issueopsdomain.DeliveryDurableRequestID(request.RetryRequestID, dispatch.Request.DurableID, dispatchFound, "dispatch")
	if err != nil {
		return request, err
	}
	request.PromptRetryRequestID, err = issueopsdomain.DeliveryDurableRequestID(request.PromptRetryRequestID, prompt.Request.DurableID, promptFound, "prompt")
	if err != nil {
		return request, err
	}
	if promptFound {
		request.ExpectedPromptProcessIncarnation = strings.TrimSpace(prompt.Target.ProcessIncarnation)
	}
	return request, nil
}

func foldedHandoffDeliveryObservation(audit Audit, request port.ExecutionOrcaIntentRequest, identity port.ExecutionOrcaDeliveryIdentity, callKind string) (issueopscontract.IssueOpsHandoffDeliveryObservation, bool, error) {
	probe := deliveryAttempt(request).Probe(handoffDeliveryLauncher(identity), callKind)
	observations, err := audit.ReadFor(probe.LifecycleID, probe.LineageID)
	return issueopsdomain.RecoverDeliveryObservation(probe, identity.TerminalPTYID, observations, err)
}

func observeHandoffDeliveryRequest(ctx context.Context, observer port.ExecutionOrcaDeliveryObserver, requestID, method, runtimeID string) (string, error) {
	if err := issueopscontract.ValidateOrcaRequestID(requestID); err != nil {
		return "", fmt.Errorf("Orca request observation identity is invalid")
	}
	observed, err := observer.ObserveRequest(ctx, requestID)
	if err != nil {
		return "", err
	}
	return issueopsdomain.DeliveryRequestStatus(requestID, method, runtimeID, observed.RequestID, observed.Method, observed.RuntimeID, observed.Status)
}

func Observation(audit Audit, request port.ExecutionOrcaIntentRequest, identity port.ExecutionOrcaDeliveryIdentity, receipt port.ExecutionOrcaIntentReceipt, durableID, callKind string, now func() time.Time) (issueopscontract.IssueOpsHandoffDeliveryObservation, error) {
	timestamp := now().UTC().Format(time.RFC3339Nano)
	attempt := deliveryAttempt(request)
	launcher := handoffDeliveryLauncher(identity)
	probe := attempt.Probe(launcher, callKind)
	observations, err := audit.ReadFor(probe.LifecycleID, probe.LineageID)
	if err != nil {
		return issueopscontract.IssueOpsHandoffDeliveryObservation{}, err
	}
	return issueopsdomain.NewDeliveryObservation(issueopsdomain.DeliveryObservationInput{Attempt: attempt, Launcher: launcher, Target: issueopscontract.IssueOpsHandoffDeliveryTarget{TerminalID: identity.TerminalPTYID, PaneID: identity.TerminalHandle}, ReceiptTarget: issueopscontract.IssueOpsHandoffDeliveryTarget{TerminalID: receipt.TerminalPTYID, PaneID: receipt.TerminalHandle}, DurableID: durableID, CallKind: callKind, Timestamp: timestamp}, observations), nil
}

func handoffDeliveryLauncher(identity port.ExecutionOrcaDeliveryIdentity) issueopscontract.IssueOpsHandoffDeliveryLauncher {
	return issueopscontract.IssueOpsHandoffDeliveryLauncher{
		Name: issueopscontract.IssueOpsHandoffDeliveryLauncherOrca, Version: strings.TrimSpace(identity.Version), Path: strings.TrimSpace(identity.LauncherPath),
		RuntimeID: strings.TrimSpace(identity.RuntimeID), MachineID: strings.TrimSpace(identity.MachineID), ServerID: strings.TrimSpace(identity.TargetIdentity),
	}
}

func handoffDeliveryCallKind(request port.ExecutionOrcaIntentRequest, err error) string {
	typed, _ := errors.AsType[*port.OrcaError](err)
	return deliveryAttempt(request).CallKind(deliveryFailure(typed))
}

func handoffDeliveryObserved(now func() time.Time, evidence string) issueopscontract.IssueOpsHandoffDeliveryState {
	return issueopsdomain.DeliveryObserved(now().UTC().Format(time.RFC3339Nano), evidence)
}

func handoffDeliveryEventClock(now func() time.Time) func() time.Time {
	timestamp := now()
	return func() time.Time { return timestamp }
}

func deliveryAttempt(r port.ExecutionOrcaIntentRequest) issueopsdomain.DeliveryAttempt {
	a := issueopsdomain.DeliveryAttempt{Stage: string(r.Stage), OperationID: r.OperationID, LifecycleID: r.Workspace.LifecycleID, Host: r.Probe.Host, SourceGeneration: r.SourceGeneration, RetryRequestID: r.RetryRequestID, PromptRetryRequestID: r.PromptRetryRequestID, TerminalID: r.TerminalPTYID, HasLaunch: r.Launch != nil}
	if r.Launch != nil {
		a.PromptSHA256 = r.Launch.PromptSHA256
		a.MaterialSHA256 = r.Launch.ContextPacketSHA256
	}
	return a
}
func deliveryFailure(f *port.OrcaError) *issueopsdomain.DeliveryFailure {
	if f == nil {
		return nil
	}
	return &issueopsdomain.DeliveryFailure{Code: f.Code, CallPhase: f.CallPhase, OrchestrationRequestID: f.OrchestrationRequestID, DispatchRequestID: f.DispatchRequestID, Invoked: f.Invoked}
}
func deliveryReceiptIdentity(r port.ExecutionOrcaIntentReceipt) issueopsdomain.DeliveryReceiptIdentity {
	return issueopsdomain.DeliveryReceiptIdentity{RequestID: r.RequestID, TaskID: r.TaskID, DispatchID: r.DispatchID, TerminalID: r.TerminalPTYID, TerminalHandle: r.TerminalHandle}
}

func deliveryPrompt(p *port.OrcaPromptReceipt) *issueopsdomain.DeliveryPrompt {
	if p == nil {
		return nil
	}
	return &issueopsdomain.DeliveryPrompt{RequestID: p.RequestID, Stages: p.Stages, ProcessIncarnation: p.ProcessIncarnation, Generation: p.Generation, BaselineWorkingSequence: p.BaselineWorkingSequence}
}
