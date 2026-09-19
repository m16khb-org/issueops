package issueopsapp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	auditadapter "issueops/internal/adapter/audit"
	issueopscontract "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

func observeHandoffDeliveryBefore(request port.ExecutionOrcaIntentRequest, now func() time.Time) error {
	if request.Launch == nil {
		return nil
	}
	observation, err := handoffDeliveryObservation(request, port.ExecutionOrcaIntentReceipt{}, "", handoffDeliveryCallKind(request, nil), now)
	if err != nil {
		return err
	}
	observation.Ambiguous = handoffDeliveryObserved(now, issueopscontract.IssueOpsHandoffDeliveryEvidenceResumeBeforeExternalCallCrash)
	_, err = auditadapter.AuditHandoffDeliveryObservation(observation)
	return err
}

func observeHandoffDeliveryAfter(request port.ExecutionOrcaIntentRequest, receipt port.ExecutionOrcaIntentReceipt, now func() time.Time) error {
	if request.Launch == nil {
		return nil
	}
	if receipt.RequestID != "" {
		observation, err := handoffDeliveryObservation(request, receipt, receipt.RequestID, "dispatch", now)
		if err != nil {
			return err
		}
		if request.Probe.Host == "omo" {
			observation.Ambiguous = handoffDeliveryObserved(now, issueopscontract.IssueOpsHandoffDeliveryEvidenceOrcaDispatchReceipt)
		} else {
			observation.InputAccepted = handoffDeliveryObserved(now, issueopscontract.IssueOpsHandoffDeliveryEvidenceOrcaDispatch)
		}
		if _, err := auditadapter.AuditHandoffDeliveryObservation(observation); err != nil {
			return err
		}
	}
	if receipt.PromptReceipt == nil {
		return nil
	}
	observation, err := handoffDeliveryObservation(request, receipt, receipt.PromptReceipt.RequestID, "prompt", now)
	if err != nil {
		return err
	}
	observation.Target.ProcessIncarnation = strings.TrimSpace(receipt.PromptReceipt.ProcessIncarnation)
	observation.InputAccepted = handoffDeliveryObserved(now, issueopscontract.IssueOpsHandoffDeliveryEvidenceOmoSendAccepted)
	if containsHandoffDeliveryString(receipt.PromptReceipt.Stages, "turn_started") {
		observation.NativeTurnObserved = handoffDeliveryObserved(now, issueopscontract.IssueOpsHandoffDeliveryEvidenceNativeReceipt)
	}
	_, err = auditadapter.AuditHandoffDeliveryObservation(observation)
	return err
}

func observeHandoffDeliveryFailure(request port.ExecutionOrcaIntentRequest, err error, now func() time.Time) error {
	if request.Launch == nil {
		return nil
	}
	callKind := handoffDeliveryCallKind(request, err)
	observation, observeErr := handoffDeliveryObservation(request, port.ExecutionOrcaIntentReceipt{}, "", callKind, now)
	if observeErr != nil {
		return observeErr
	}
	if typed, ok := errors.AsType[*port.OrcaError](err); ok {
		observation.Request.DurableID = strings.TrimSpace(typed.OrchestrationRequestID)
		if typed.CallPhase == "terminal_send" {
			if strings.TrimSpace(typed.DispatchRequestID) != "" {
				dispatchObservation, dispatchErr := handoffDeliveryObservation(request, port.ExecutionOrcaIntentReceipt{}, strings.TrimSpace(typed.DispatchRequestID), "dispatch", now)
				if dispatchErr != nil {
					return dispatchErr
				}
				dispatchObservation.Ambiguous = handoffDeliveryObserved(now, issueopscontract.IssueOpsHandoffDeliveryEvidenceOrcaDispatchReceipt)
				if _, dispatchErr := auditadapter.AuditHandoffDeliveryObservation(dispatchObservation); dispatchErr != nil {
					return dispatchErr
				}
			}
			observation.Ambiguous = handoffDeliveryObserved(now, issueopscontract.IssueOpsHandoffDeliveryEvidenceOmoSendResponseLost)
		} else {
			observation.Ambiguous = handoffDeliveryObserved(now, issueopscontract.IssueOpsHandoffDeliveryEvidenceAcceptedResponseLost)
		}
	} else {
		observation.Ambiguous = handoffDeliveryObserved(now, issueopscontract.IssueOpsHandoffDeliveryEvidenceAcceptedResponseLost)
	}
	_, observeErr = auditadapter.AuditHandoffDeliveryObservation(observation)
	return observeErr
}

func consumeHandoffDeliveryRecoveryEvidence(request port.ExecutionOrcaIntentRequest) error {
	if request.Launch == nil {
		return nil
	}
	folded, decisions, err := auditadapter.FoldHandoffDeliveryAuditObservations()
	for _, decision := range decisions {
		if !decision.Accepted {
			if err != nil {
				return fmt.Errorf("handoff delivery recovery evidence rejected: %w", err)
			}
			return fmt.Errorf("handoff delivery recovery evidence rejected: %s", strings.Join(decision.RejectReasons, "; "))
		}
	}
	if err != nil {
		return err
	}
	probe := handoffDeliveryObservationProbe(request, handoffDeliveryCallKind(request, nil))
	key := issueopsdomain.HandoffDeliveryFoldKey(probe)
	if key == "" {
		return nil
	}
	observation, ok := folded[key]
	if !ok {
		return nil
	}
	if observation.PromptSHA256 != probe.PromptSHA256 || observation.MaterialSHA256 != probe.MaterialSHA256 || observation.SourceGeneration != probe.SourceGeneration || observation.Launcher.Name != probe.Launcher.Name || observation.Launcher.RuntimeID != probe.Launcher.RuntimeID {
		return fmt.Errorf("handoff delivery recovery evidence conflicts with current request identity")
	}
	return nil
}

func handoffDeliveryObservation(request port.ExecutionOrcaIntentRequest, receipt port.ExecutionOrcaIntentReceipt, durableID, callKind string, now func() time.Time) (issueopscontract.IssueOpsHandoffDeliveryObservation, error) {
	timestamp := now().UTC().Format(time.RFC3339Nano)
	createdAt := timestamp
	probe := handoffDeliveryObservationProbe(request, callKind)
	folded, _, err := auditadapter.FoldHandoffDeliveryAuditObservations()
	if err != nil {
		return issueopscontract.IssueOpsHandoffDeliveryObservation{}, err
	}
	if current, ok := folded[issueopsdomain.HandoffDeliveryFoldKey(probe)]; ok {
		createdAt = current.CreatedAt
	}
	machine, _ := os.Hostname()
	if strings.TrimSpace(machine) == "" {
		machine = "unknown"
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
		Launcher:          handoffDeliveryLauncher(request, machine),
		Target: issueopscontract.IssueOpsHandoffDeliveryTarget{
			TerminalID: firstNonEmptyIssueOpsApp(receipt.TerminalPTYID, request.TerminalPTYID),
			PaneID:     strings.TrimSpace(receipt.TerminalHandle),
		},
		SourceGeneration: request.SourceGeneration,
		CreatedAt:        createdAt,
		UpdatedAt:        timestamp,
		Receipt: issueopscontract.IssueOpsHandoffDeliveryReceipt{
			Location: "audit/handoff-delivery.jsonl#" + strings.TrimSpace(request.OperationID) + ":" + callKind,
			Digest:   digestHandoffDeliveryValue(request, receipt),
		},
		InputAccepted:      issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateNotObserved},
		NativeTurnObserved: issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateNotObserved},
		OwnerClaimed:       issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateNotObserved},
		Ambiguous:          issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateNotObserved},
	}, nil
}

func handoffDeliveryObservationProbe(request port.ExecutionOrcaIntentRequest, callKind string) issueopscontract.IssueOpsHandoffDeliveryObservation {
	machine, _ := os.Hostname()
	if strings.TrimSpace(machine) == "" {
		machine = "unknown"
	}
	return issueopscontract.IssueOpsHandoffDeliveryObservation{
		LineageID:         handoffDeliveryLineageID(request, callKind),
		LifecycleID:       strings.TrimSpace(request.Workspace.LifecycleID),
		PromptSHA256:      strings.TrimSpace(request.Launch.PromptSHA256),
		MaterialSHA256:    strings.TrimSpace(request.Launch.ContextPacketSHA256),
		SourceGeneration:  request.SourceGeneration,
		ExpectedOwnerHost: strings.TrimSpace(request.Probe.Host),
		Launcher:          handoffDeliveryLauncher(request, machine),
	}
}

func handoffDeliveryLauncher(request port.ExecutionOrcaIntentRequest, machine string) issueopscontract.IssueOpsHandoffDeliveryLauncher {
	runtimeID := "pending"
	if request.Prepared != nil && strings.TrimSpace(request.Prepared.RuntimeID) != "" {
		runtimeID = strings.TrimSpace(request.Prepared.RuntimeID)
	}
	path := "orca"
	if resolved, err := exec.LookPath("orca"); err == nil && strings.TrimSpace(resolved) != "" {
		path = resolved
	}
	return issueopscontract.IssueOpsHandoffDeliveryLauncher{
		Name: issueopscontract.IssueOpsHandoffDeliveryLauncherOrca, Version: "unknown", Path: path,
		RuntimeID: runtimeID, MachineID: strings.TrimSpace(machine), ServerID: runtimeID,
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

func digestHandoffDeliveryValue(values ...any) string {
	data, _ := json.Marshal(values)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
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
	return "pending"
}
