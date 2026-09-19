package issueops

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
)

var handoffDeliveryDigest = regexp.MustCompile(`^[a-f0-9]{64}$`)

const handoffDeliveryFieldLimit = 1024

func ValidateHandoffDeliveryObservation(observation issueopscontract.IssueOpsHandoffDeliveryObservation) error {
	if observation.SchemaVersion != issueopscontract.IssueOpsHandoffDeliverySchemaVersion {
		return fmt.Errorf("unsupported delivery observation schema")
	}
	if strings.TrimSpace(observation.AttemptID) == "" {
		return fmt.Errorf("delivery observation attempt_id is required")
	}
	if strings.TrimSpace(observation.LifecycleID) == "" {
		return fmt.Errorf("delivery observation lifecycle_id is required")
	}
	if !handoffDeliveryDigest.MatchString(strings.TrimSpace(observation.PromptSHA256)) {
		return fmt.Errorf("delivery observation prompt digest is invalid")
	}
	if err := validateHandoffDeliveryRequest(observation.Request, observation.AttemptID); err != nil {
		return err
	}
	if observation.SourceGeneration == 0 {
		return fmt.Errorf("delivery observation source generation is required")
	}
	createdAt, updatedAt, err := validateHandoffDeliveryTimestamps(observation.CreatedAt, observation.UpdatedAt)
	if err != nil {
		return err
	}
	if strings.TrimSpace(observation.Receipt.Location) == "" || !handoffDeliveryDigest.MatchString(strings.TrimSpace(observation.Receipt.Digest)) {
		return fmt.Errorf("delivery observation receipt is invalid")
	}
	if len(observation.Receipt.Location) > handoffDeliveryFieldLimit {
		return fmt.Errorf("delivery observation receipt location is too large")
	}
	if err := validateHandoffDeliveryLauncher(observation.Launcher); err != nil {
		return err
	}
	if err := validateHandoffDeliveryTarget(observation.Target); err != nil {
		return err
	}
	for _, item := range []struct {
		name  string
		state issueopscontract.IssueOpsHandoffDeliveryState
	}{
		{"input_accepted", observation.InputAccepted},
		{"native_turn_observed", observation.NativeTurnObserved},
		{"owner_claimed", observation.OwnerClaimed},
		{"ambiguous", observation.Ambiguous},
	} {
		if err := validateHandoffDeliveryState(item.name, item.state, observation); err != nil {
			return err
		}
		if err := validateHandoffDeliveryStateTimestamp(item.name, item.state, createdAt, updatedAt); err != nil {
			return err
		}
	}
	if observation.OwnerActor != nil {
		if err := issueopscontract.ValidateNativeActor(*observation.OwnerActor); err != nil {
			return err
		}
	}
	if err := validateHandoffDeliveryClaimConsistency(observation); err != nil {
		return err
	}
	if err := validateHandoffDeliveryModeEvidence(observation); err != nil {
		return err
	}
	return nil
}

func FoldHandoffDeliveryObservations(observations []issueopscontract.IssueOpsHandoffDeliveryObservation) (map[string]issueopscontract.IssueOpsHandoffDeliveryObservation, []issueopscontract.IssueOpsHandoffDeliveryDecision) {
	folded := map[string]issueopscontract.IssueOpsHandoffDeliveryObservation{}
	decisions := []issueopscontract.IssueOpsHandoffDeliveryDecision{}
	for _, observation := range observations {
		attemptID := strings.TrimSpace(observation.AttemptID)
		if attemptID == "" {
			decisions = append(decisions, handoffDeliveryReject("delivery observation attempt_id is required"))
			continue
		}
		current, ok := folded[attemptID]
		if !ok {
			if err := ValidateHandoffDeliveryObservation(observation); err != nil {
				decisions = append(decisions, handoffDeliveryReject(err.Error()))
				continue
			}
			folded[attemptID] = observation
			decisions = append(decisions, issueopscontract.IssueOpsHandoffDeliveryDecision{Accepted: true})
			continue
		}
		merged, decision := MergeHandoffDeliveryObservation(current, observation)
		decisions = append(decisions, decision)
		if decision.Accepted {
			folded[attemptID] = merged
		}
	}
	return folded, decisions
}

func MergeHandoffDeliveryObservation(current, next issueopscontract.IssueOpsHandoffDeliveryObservation) (issueopscontract.IssueOpsHandoffDeliveryObservation, issueopscontract.IssueOpsHandoffDeliveryDecision) {
	if err := ValidateHandoffDeliveryObservation(current); err != nil {
		return current, handoffDeliveryReject(err.Error())
	}
	if err := ValidateHandoffDeliveryObservation(next); err != nil {
		return current, handoffDeliveryReject(err.Error())
	}
	if reason := handoffDeliveryIdentityMismatch(current, next); reason != "" {
		return current, handoffDeliveryReject(reason)
	}
	currentUpdatedAt, _ := validateHandoffDeliveryTime(current.UpdatedAt)
	nextUpdatedAt, _ := validateHandoffDeliveryTime(next.UpdatedAt)
	if nextUpdatedAt.Before(currentUpdatedAt) {
		return current, handoffDeliveryReject("delivery observation update timestamp moved backward")
	}
	if current.Launcher.Name == "cmux" && next.NativeTurnObserved.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved &&
		next.NativeTurnObserved.Evidence == issueopscontract.IssueOpsHandoffDeliveryEvidenceRawInput {
		return current, handoffDeliveryReject("cmux raw input cannot prove native turn")
	}
	if next.OwnerClaimed.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved {
		if err := validateHandoffDeliveryOwnerClaim(current, next.OwnerClaim); err != nil {
			return current, handoffDeliveryReject(err.Error())
		}
	}
	merged := current
	merged.UpdatedAt = next.UpdatedAt
	merged.InputAccepted = mergeHandoffDeliveryState(merged.InputAccepted, next.InputAccepted)
	merged.NativeTurnObserved = mergeHandoffDeliveryState(merged.NativeTurnObserved, next.NativeTurnObserved)
	merged.OwnerClaimed = mergeHandoffDeliveryState(merged.OwnerClaimed, next.OwnerClaimed)
	merged.Ambiguous = mergeHandoffDeliveryState(merged.Ambiguous, next.Ambiguous)
	if merged.Request.RetryRequestID == "" && next.Request.RetryRequestID != "" {
		merged.Request.RetryRequestID = next.Request.RetryRequestID
		merged.Request.RetryOfAttempt = next.Request.RetryOfAttempt
	}
	if next.OwnerClaim.Claimed {
		merged.OwnerClaim = next.OwnerClaim
	}
	return merged, issueopscontract.IssueOpsHandoffDeliveryDecision{Accepted: true}
}

func validateHandoffDeliveryLauncher(launcher issueopscontract.IssueOpsHandoffDeliveryLauncher) error {
	switch launcher.Name {
	case issueopscontract.IssueOpsHandoffDeliveryLauncherDirect, issueopscontract.IssueOpsHandoffDeliveryLauncherOrca,
		issueopscontract.IssueOpsHandoffDeliveryLauncherHerdr, issueopscontract.IssueOpsHandoffDeliveryLauncherCmux:
	default:
		return fmt.Errorf("delivery observation launcher is invalid")
	}
	for name, value := range map[string]string{
		"launcher name":       launcher.Name,
		"launcher version":    launcher.Version,
		"launcher path":       launcher.Path,
		"launcher runtime_id": launcher.RuntimeID,
		"launcher machine_id": launcher.MachineID,
		"launcher server_id":  launcher.ServerID,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("delivery observation %s is required", name)
		}
		if len(value) > handoffDeliveryFieldLimit {
			return fmt.Errorf("delivery observation %s is too large", name)
		}
	}
	return nil
}

func validateHandoffDeliveryTarget(target issueopscontract.IssueOpsHandoffDeliveryTarget) error {
	if strings.TrimSpace(target.TerminalID) == "" && strings.TrimSpace(target.PaneID) == "" {
		return fmt.Errorf("delivery observation terminal or pane identity is required")
	}
	if target.Process.PID <= 0 || strings.TrimSpace(target.Process.StartedAt) == "" || strings.TrimSpace(target.Process.Executable) == "" {
		return fmt.Errorf("delivery observation process identity is required")
	}
	return nil
}

func validateHandoffDeliveryState(name string, state issueopscontract.IssueOpsHandoffDeliveryState, observation issueopscontract.IssueOpsHandoffDeliveryObservation) error {
	switch state.Status {
	case issueopscontract.IssueOpsHandoffDeliveryStateNotObserved:
		return nil
	case issueopscontract.IssueOpsHandoffDeliveryStateObserved:
		if strings.TrimSpace(state.ObservedAt) == "" || strings.TrimSpace(state.Evidence) == "" {
			return fmt.Errorf("delivery observation %s evidence is required", name)
		}
		if !validHandoffDeliveryEvidenceForState(name, state.Evidence, observation) {
			return fmt.Errorf("delivery observation %s evidence is invalid", name)
		}
		return nil
	default:
		return fmt.Errorf("delivery observation %s state is invalid", name)
	}
}

func handoffDeliveryIdentityMismatch(current, next issueopscontract.IssueOpsHandoffDeliveryObservation) string {
	requestReason := handoffDeliveryRequestIdentityMismatch(current.Request, next.Request)
	switch {
	case current.AttemptID != next.AttemptID:
		return "delivery observation attempt identity changed"
	case current.LifecycleID != next.LifecycleID:
		return "delivery observation lifecycle identity changed"
	case current.PromptSHA256 != next.PromptSHA256:
		return "delivery observation prompt digest changed"
	case requestReason != "":
		return requestReason
	case current.CreatedAt != next.CreatedAt:
		return "delivery observation created timestamp changed"
	case current.SourceGeneration != next.SourceGeneration:
		return "delivery observation source generation changed"
	case !reflect.DeepEqual(current.Launcher, next.Launcher):
		return "delivery observation launcher identity changed"
	case !reflect.DeepEqual(current.Target, next.Target):
		return "delivery observation process identity changed"
	case !reflect.DeepEqual(current.OwnerActor, next.OwnerActor):
		return "delivery observation owner identity changed"
	case !reflect.DeepEqual(current.Receipt, next.Receipt):
		return "delivery observation receipt identity changed"
	default:
		return ""
	}
}

func validateHandoffDeliveryOwnerClaim(current issueopscontract.IssueOpsHandoffDeliveryObservation, claim issueopscontract.IssueOpsHandoffDeliveryOwnerClaim) error {
	if !claim.Claimed || claim.Generation != current.SourceGeneration || strings.TrimSpace(claim.ClaimedAt) == "" || current.OwnerActor == nil {
		return fmt.Errorf("delivery owner claim identity mismatch")
	}
	if !reflect.DeepEqual(claim.Actor, *current.OwnerActor) {
		return fmt.Errorf("delivery owner claim identity mismatch")
	}
	return nil
}

func handoffDeliveryRequestIdentityMismatch(current, next issueopscontract.IssueOpsHandoffDeliveryRequest) string {
	if current.DurableID != next.DurableID {
		return "delivery observation request identity changed"
	}
	if current.RetryRequestID != "" && next.RetryRequestID != "" && current.RetryRequestID != next.RetryRequestID {
		return "delivery observation retry request identity changed"
	}
	if current.RetryOfAttempt != "" && next.RetryOfAttempt != "" && current.RetryOfAttempt != next.RetryOfAttempt {
		return "delivery observation retry request identity changed"
	}
	return ""
}

func validateHandoffDeliveryClaimConsistency(observation issueopscontract.IssueOpsHandoffDeliveryObservation) error {
	observed := observation.OwnerClaimed.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved
	claimed := observation.OwnerClaim.Claimed
	if observed != claimed {
		return fmt.Errorf("delivery owner claim state is inconsistent")
	}
	if !observed {
		return nil
	}
	if observation.OwnerClaimed.Evidence != issueopscontract.IssueOpsHandoffDeliveryEvidenceIssueOpsClaim {
		return fmt.Errorf("delivery owner claim evidence is invalid")
	}
	if _, err := validateHandoffDeliveryTime(observation.OwnerClaim.ClaimedAt); err != nil {
		return fmt.Errorf("delivery owner claim timestamp is invalid")
	}
	createdAt, updatedAt, _ := validateHandoffDeliveryTimestamps(observation.CreatedAt, observation.UpdatedAt)
	claimedAt, _ := validateHandoffDeliveryTime(observation.OwnerClaim.ClaimedAt)
	if claimedAt.Before(createdAt) || claimedAt.After(updatedAt) {
		return fmt.Errorf("delivery owner claim timestamp is not monotonic")
	}
	if err := validateHandoffDeliveryOwnerClaim(observation, observation.OwnerClaim); err != nil {
		return err
	}
	return nil
}

func mergeHandoffDeliveryState(current, next issueopscontract.IssueOpsHandoffDeliveryState) issueopscontract.IssueOpsHandoffDeliveryState {
	if current.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved {
		return current
	}
	if next.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved {
		return next
	}
	return current
}

func validateHandoffDeliveryRequest(request issueopscontract.IssueOpsHandoffDeliveryRequest, attemptID string) error {
	if strings.TrimSpace(request.DurableID) == "" || len(request.DurableID) > handoffDeliveryFieldLimit {
		return fmt.Errorf("delivery observation durable request identity is invalid")
	}
	if request.RetryRequestID != "" {
		if len(request.RetryRequestID) > handoffDeliveryFieldLimit || strings.TrimSpace(request.RetryOfAttempt) != strings.TrimSpace(attemptID) {
			return fmt.Errorf("delivery observation retry request identity is invalid")
		}
	}
	return nil
}

func validateHandoffDeliveryTimestamps(created, updated string) (time.Time, time.Time, error) {
	createdAt, err := validateHandoffDeliveryTime(created)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("delivery observation created_at is invalid")
	}
	updatedAt, err := validateHandoffDeliveryTime(updated)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("delivery observation updated_at is invalid")
	}
	if updatedAt.Before(createdAt) {
		return time.Time{}, time.Time{}, fmt.Errorf("delivery observation timestamps are not monotonic")
	}
	return createdAt, updatedAt, nil
}

func validateHandoffDeliveryStateTimestamp(name string, state issueopscontract.IssueOpsHandoffDeliveryState, createdAt, updatedAt time.Time) error {
	if state.Status != issueopscontract.IssueOpsHandoffDeliveryStateObserved {
		return nil
	}
	observedAt, err := validateHandoffDeliveryTime(state.ObservedAt)
	if err != nil {
		return fmt.Errorf("delivery observation %s timestamp is invalid", name)
	}
	if observedAt.Before(createdAt) || observedAt.After(updatedAt) {
		return fmt.Errorf("delivery observation %s timestamp is not monotonic", name)
	}
	return nil
}

func validateHandoffDeliveryTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
}

func validateHandoffDeliveryModeEvidence(observation issueopscontract.IssueOpsHandoffDeliveryObservation) error {
	if observation.Launcher.Name == issueopscontract.IssueOpsHandoffDeliveryLauncherHerdr &&
		observation.NativeTurnObserved.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved &&
		observation.NativeTurnObserved.Evidence == issueopscontract.IssueOpsHandoffDeliveryEvidenceHerdrWaitState {
		return fmt.Errorf("Herdr wait state cannot prove native turn")
	}
	if observation.Launcher.Name == issueopscontract.IssueOpsHandoffDeliveryLauncherCmux &&
		observation.NativeTurnObserved.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved &&
		observation.NativeTurnObserved.Evidence == issueopscontract.IssueOpsHandoffDeliveryEvidenceRawInput {
		return fmt.Errorf("cmux raw input cannot prove native turn")
	}
	return nil
}

func validHandoffDeliveryEvidenceForState(stateName, evidence string, observation issueopscontract.IssueOpsHandoffDeliveryObservation) bool {
	switch stateName {
	case "input_accepted":
		switch evidence {
		case issueopscontract.IssueOpsHandoffDeliveryEvidenceLauncherReceipt,
			issueopscontract.IssueOpsHandoffDeliveryEvidenceLauncherAccepted,
			issueopscontract.IssueOpsHandoffDeliveryEvidenceOrcaDispatch,
			issueopscontract.IssueOpsHandoffDeliveryEvidenceRawInput:
			return true
		case issueopscontract.IssueOpsHandoffDeliveryEvidenceOmoSendAccepted:
			return handoffDeliveryOmoEvidenceAllowed(observation)
		default:
			return false
		}
	case "native_turn_observed":
		return evidence == issueopscontract.IssueOpsHandoffDeliveryEvidenceNativeReceipt
	case "owner_claimed":
		return evidence == issueopscontract.IssueOpsHandoffDeliveryEvidenceIssueOpsClaim
	case "ambiguous":
		switch evidence {
		case issueopscontract.IssueOpsHandoffDeliveryEvidenceAcceptedResponseLost,
			issueopscontract.IssueOpsHandoffDeliveryEvidenceTimeout,
			issueopscontract.IssueOpsHandoffDeliveryEvidenceAgentPromptStalled,
			issueopscontract.IssueOpsHandoffDeliveryEvidenceHerdrWaitState,
			issueopscontract.IssueOpsHandoffDeliveryEvidenceReplaceBeforeExternalCallCrash,
			issueopscontract.IssueOpsHandoffDeliveryEvidenceReplaceAfterExternalCallCrash,
			issueopscontract.IssueOpsHandoffDeliveryEvidenceReseedBeforeExternalCallCrash,
			issueopscontract.IssueOpsHandoffDeliveryEvidenceReseedAfterExternalCallCrash,
			issueopscontract.IssueOpsHandoffDeliveryEvidenceResumeBeforeExternalCallCrash,
			issueopscontract.IssueOpsHandoffDeliveryEvidenceResumeAfterExternalCallCrash:
			return true
		case issueopscontract.IssueOpsHandoffDeliveryEvidenceOmoSendFailed,
			issueopscontract.IssueOpsHandoffDeliveryEvidenceOmoSendResponseLost:
			return handoffDeliveryOmoEvidenceAllowed(observation)
		default:
			return false
		}
	default:
		return false
	}
}

func handoffDeliveryOmoEvidenceAllowed(observation issueopscontract.IssueOpsHandoffDeliveryObservation) bool {
	return observation.Launcher.Name == issueopscontract.IssueOpsHandoffDeliveryLauncherOrca &&
		observation.OwnerActor != nil && observation.OwnerActor.Host == "omo"
}

func handoffDeliveryReject(reason string) issueopscontract.IssueOpsHandoffDeliveryDecision {
	return issueopscontract.IssueOpsHandoffDeliveryDecision{RejectReasons: []string{reason}}
}
