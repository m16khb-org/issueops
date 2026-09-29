package issueops

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

type DeliveryRecoveryPlan struct {
	DispatchID, PromptID string
	Active               bool
}

func PlanDeliveryRecovery(a DeliveryAttempt, dispatch, prompt model.IssueOpsHandoffDeliveryObservation, dispatchFound, promptFound bool) (DeliveryRecoveryPlan, error) {
	var p DeliveryRecoveryPlan
	var err error
	p.DispatchID, err = DeliveryDurableRequestID(a.RetryRequestID, dispatch.Request.DurableID, dispatchFound, "dispatch")
	if err != nil {
		return p, err
	}
	if dispatchFound && p.DispatchID == "" {
		return p, fmt.Errorf("Orca dispatch delivery is ambiguous without a durable request ID")
	}
	p.PromptID, err = DeliveryDurableRequestID(a.PromptRetryRequestID, prompt.Request.DurableID, promptFound, "prompt")
	if err != nil {
		return p, err
	}
	p.Active = dispatchFound || promptFound
	if !p.Active {
		return p, nil
	}
	if a.Host == "omo" && promptFound && p.PromptID == "" {
		return p, fmt.Errorf("Omo prompt delivery is ambiguous without a durable request ID")
	}
	return p, nil
}
func InspectDeliveryDispatch(host string, dispatchFound bool, status string) (bool, error) {
	if host != "omo" {
		return status == "completed" || status == "pending", nil
	}
	if !dispatchFound {
		return false, fmt.Errorf("Omo prompt delivery has no recovered dispatch lineage")
	}
	return true, nil
}

type DeliveryRecoveryMode string

const (
	DeliveryRecoveryDelegate  DeliveryRecoveryMode = "delegate"
	DeliveryRecoveryReplay    DeliveryRecoveryMode = "replay"
	DeliveryRecoveryCandidate DeliveryRecoveryMode = "candidate"
)

type DeliveryRecoveryConclusion struct {
	Mode   DeliveryRecoveryMode
	Prompt *DeliveryPrompt
}

func FinishDeliveryRecovery(host string, plan DeliveryRecoveryPlan, dispatchStatus string, dispatchExists bool, prompt model.IssueOpsHandoffDeliveryObservation, promptFound bool) (DeliveryRecoveryConclusion, error) {
	if host != "omo" {
		if dispatchExists {
			return DeliveryRecoveryConclusion{Mode: DeliveryRecoveryCandidate}, nil
		}
		return DeliveryRecoveryConclusion{Mode: DeliveryRecoveryReplay}, nil
	}
	if promptFound && (prompt.InputAccepted.Status == model.IssueOpsHandoffDeliveryStateObserved || prompt.OwnerClaimed.Status == model.IssueOpsHandoffDeliveryStateObserved) {
		if !dispatchExists || strings.TrimSpace(prompt.Request.DurableID) == "" || strings.TrimSpace(prompt.Target.ProcessIncarnation) == "" || prompt.Target.PromptGeneration == nil || prompt.Target.BaselineWorkingSequence == nil {
			return DeliveryRecoveryConclusion{}, fmt.Errorf("Omo prompt delivery recovery evidence is incomplete")
		}
		stages := []string{"input_accepted"}
		if prompt.NativeTurnObserved.Status == model.IssueOpsHandoffDeliveryStateObserved {
			stages = append(stages, "turn_started")
		}
		return DeliveryRecoveryConclusion{Mode: DeliveryRecoveryCandidate, Prompt: &DeliveryPrompt{RequestID: prompt.Request.DurableID, Stages: stages, ProcessIncarnation: prompt.Target.ProcessIncarnation, Generation: *prompt.Target.PromptGeneration, BaselineWorkingSequence: cloneDeliveryUint64(prompt.Target.BaselineWorkingSequence)}}, nil
	}
	// Terminal prompt UUIDs are replayed through terminal send, never request-show.
	if plan.PromptID != "" || plan.DispatchID != "" && (dispatchStatus == "completed" || dispatchStatus == "pending") {
		return DeliveryRecoveryConclusion{Mode: DeliveryRecoveryReplay}, nil
	}
	return DeliveryRecoveryConclusion{Mode: DeliveryRecoveryDelegate}, nil
}
