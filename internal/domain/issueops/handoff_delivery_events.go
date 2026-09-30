package issueops

import (
	"fmt"
	"slices"
	"strings"

	model "issueops/internal/contract/issueops"
)

type DeliveryPrompt struct {
	RequestID, ProcessIncarnation string
	Stages                        []string
	Generation                    uint64
	BaselineWorkingSequence       *uint64
}

func DeliveryObserved(timestamp, evidence string) model.IssueOpsHandoffDeliveryState {
	return model.IssueOpsHandoffDeliveryState{Status: model.IssueOpsHandoffDeliveryStateObserved, ObservedAt: timestamp, Evidence: evidence}
}
func DeliveryCompletionID(dispatchID, callKind string, prompt *DeliveryPrompt) string {
	if callKind == "prompt" && prompt != nil {
		return strings.TrimSpace(prompt.RequestID)
	}
	return strings.TrimSpace(dispatchID)
}
func CompleteDeliveryObservation(o model.IssueOpsHandoffDeliveryObservation, host, callKind, timestamp string, prompt *DeliveryPrompt) (model.IssueOpsHandoffDeliveryObservation, error) {
	if callKind == "dispatch" {
		if host == "omo" {
			o.Ambiguous = DeliveryObserved(timestamp, model.IssueOpsHandoffDeliveryEvidenceOrcaDispatchReceipt)
		} else {
			o.InputAccepted = DeliveryObserved(timestamp, model.IssueOpsHandoffDeliveryEvidenceOrcaDispatch)
		}
	} else {
		if prompt == nil {
			return o, fmt.Errorf("Omo prompt completion is missing its durable receipt")
		}
		o.Target.ProcessIncarnation = strings.TrimSpace(prompt.ProcessIncarnation)
		generation := prompt.Generation
		o.Target.PromptGeneration = &generation
		o.Target.BaselineWorkingSequence = cloneDeliveryUint64(prompt.BaselineWorkingSequence)
		o.InputAccepted = DeliveryObserved(timestamp, model.IssueOpsHandoffDeliveryEvidenceOmoSendAccepted)
		if slices.Contains(prompt.Stages, "turn_started") {
			o.NativeTurnObserved = DeliveryObserved(timestamp, model.IssueOpsHandoffDeliveryEvidenceNativeReceipt)
		}
	}
	return o, nil
}

type DeliveryFailureDisposition struct {
	Skip                            bool
	DurableID, DispatchID, Evidence string
}

func DeliveryFailurePlan(a DeliveryAttempt, failure *DeliveryFailure) DeliveryFailureDisposition {
	p := DeliveryFailureDisposition{Evidence: model.IssueOpsHandoffDeliveryEvidenceAcceptedResponseLost}
	if failure == nil {
		return p
	}
	if !failure.Invoked || failure.Code == "delivery_observation_failed" || DeliveryResponseIdentityRejected(a, failure) {
		return DeliveryFailureDisposition{Skip: true}
	}
	p.DurableID = DeliveryAcceptedResponseID("", failure.OrchestrationRequestID)
	if failure.CallPhase == "terminal_send" {
		p.DispatchID = DeliveryAcceptedResponseID(a.RetryRequestID, failure.DispatchRequestID)
		p.Evidence = model.IssueOpsHandoffDeliveryEvidenceOmoSendResponseLost
	}
	return p
}

type DeliveryObservationInput struct {
	Attempt                        DeliveryAttempt
	Launcher                       model.IssueOpsHandoffDeliveryLauncher
	Target, ReceiptTarget          model.IssueOpsHandoffDeliveryTarget
	DurableID, CallKind, Timestamp string
}

func NewDeliveryObservation(input DeliveryObservationInput, observations []model.IssueOpsHandoffDeliveryObservation) model.IssueOpsHandoffDeliveryObservation {
	a := input.Attempt
	probe := a.Probe(input.Launcher, input.CallKind)
	createdAt := input.Timestamp
	folded, _ := FoldHandoffDeliveryObservations(observations)
	if current, ok := folded[HandoffDeliveryFoldKey(probe)]; ok {
		createdAt = current.CreatedAt
	}
	probe.SchemaVersion = model.IssueOpsHandoffDeliverySchemaVersion
	probe.AttemptID = a.AttemptID(input.CallKind)
	probe.Request = model.IssueOpsHandoffDeliveryRequest{DurableID: strings.TrimSpace(input.DurableID)}
	probe.Target = model.IssueOpsHandoffDeliveryTarget{TerminalID: firstDeliveryValue(input.ReceiptTarget.TerminalID, input.Target.TerminalID, a.TerminalID), PaneID: firstDeliveryValue(input.ReceiptTarget.PaneID, input.Target.PaneID)}
	probe.CreatedAt = createdAt
	probe.UpdatedAt = input.Timestamp
	probe.InputAccepted = model.IssueOpsHandoffDeliveryState{Status: model.IssueOpsHandoffDeliveryStateNotObserved}
	probe.NativeTurnObserved = model.IssueOpsHandoffDeliveryState{Status: model.IssueOpsHandoffDeliveryStateNotObserved}
	probe.OwnerClaimed = model.IssueOpsHandoffDeliveryState{Status: model.IssueOpsHandoffDeliveryStateNotObserved}
	probe.Ambiguous = model.IssueOpsHandoffDeliveryState{Status: model.IssueOpsHandoffDeliveryStateNotObserved}
	return probe
}
func cloneDeliveryUint64(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
func firstDeliveryValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
