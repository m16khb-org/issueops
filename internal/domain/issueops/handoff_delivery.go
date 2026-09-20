package issueops

import (
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
)

var handoffDeliveryDigest = regexp.MustCompile(`^[a-f0-9]{64}$`)

const handoffDeliveryFieldLimit = 1024

const handoffDeliveryManualLineagePrefix = "manual-direct:"

func ValidateHandoffDeliveryObservation(observation issueopscontract.IssueOpsHandoffDeliveryObservation) error {
	if observation.SchemaVersion != issueopscontract.IssueOpsHandoffDeliverySchemaVersion {
		return fmt.Errorf("unsupported delivery observation schema")
	}
	if strings.TrimSpace(observation.AttemptID) == "" || len(observation.AttemptID) > handoffDeliveryFieldLimit {
		return fmt.Errorf("delivery observation attempt_id is required")
	}
	if strings.TrimSpace(observation.LineageID) == "" || len(observation.LineageID) > handoffDeliveryFieldLimit {
		return fmt.Errorf("delivery observation lineage_id is required")
	}
	if strings.TrimSpace(observation.LifecycleID) == "" || len(observation.LifecycleID) > handoffDeliveryFieldLimit {
		return fmt.Errorf("delivery observation lifecycle_id is required")
	}
	if !handoffDeliveryDigest.MatchString(strings.TrimSpace(observation.PromptSHA256)) {
		return fmt.Errorf("delivery observation prompt digest is invalid")
	}
	if !handoffDeliveryDigest.MatchString(strings.TrimSpace(observation.MaterialSHA256)) {
		return fmt.Errorf("delivery observation material digest is invalid")
	}
	if err := validateHandoffDeliveryRequest(observation.Request); err != nil {
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
	if err := validateHandoffDeliveryTarget(observation); err != nil {
		return err
	}
	if err := validateHandoffDeliveryTiming(observation.Timing); err != nil {
		return err
	}
	for _, item := range []struct {
		name  string
		state issueopscontract.IssueOpsHandoffDeliveryState
	}{
		{"call_staged", observation.CallStaged},
		{"input_accepted", observation.InputAccepted},
		{"native_turn_observed", observation.NativeTurnObserved},
		{"owner_claimed", observation.OwnerClaimed},
		{"ambiguous", observation.Ambiguous},
	} {
		if item.name == "call_staged" && item.state.Status == "" {
			continue
		}
		if err := validateHandoffDeliveryState(item.name, item.state, observation); err != nil {
			return err
		}
		if err := validateHandoffDeliveryStateTimestamp(item.name, item.state, createdAt, updatedAt); err != nil {
			return err
		}
	}
	if observation.OwnerActor != nil {
		if err := validateHandoffDeliveryActorBounds(*observation.OwnerActor); err != nil {
			return err
		}
		if err := issueopscontract.ValidateNativeActor(*observation.OwnerActor); err != nil {
			return err
		}
	}
	if observation.ExpectedOwnerHost != "" && observation.ExpectedOwnerHost != "codex" && observation.ExpectedOwnerHost != "claude" && observation.ExpectedOwnerHost != "omo" {
		return fmt.Errorf("delivery observation expected owner host is invalid")
	}
	if err := validateHandoffDeliveryClaimConsistency(observation); err != nil {
		return err
	}
	if err := validateHandoffDeliveryModeEvidence(observation); err != nil {
		return err
	}
	if !handoffDeliveryHasEvidence(observation) {
		return fmt.Errorf("delivery observation requires at least one observed evidence state")
	}
	return nil
}

func FoldHandoffDeliveryObservations(observations []issueopscontract.IssueOpsHandoffDeliveryObservation) (map[string]issueopscontract.IssueOpsHandoffDeliveryObservation, []issueopscontract.IssueOpsHandoffDeliveryDecision) {
	folded := map[string]issueopscontract.IssueOpsHandoffDeliveryObservation{}
	decisions := []issueopscontract.IssueOpsHandoffDeliveryDecision{}
	for _, observation := range observations {
		key := HandoffDeliveryFoldKey(observation)
		if key == "" {
			decisions = append(decisions, handoffDeliveryReject("delivery observation lineage identity is required"))
			continue
		}
		current, ok := folded[key]
		if !ok {
			if err := ValidateHandoffDeliveryObservation(observation); err != nil {
				decisions = append(decisions, handoffDeliveryReject(err.Error()))
				continue
			}
			folded[key] = observation
			decisions = append(decisions, issueopscontract.IssueOpsHandoffDeliveryDecision{Accepted: true})
			continue
		}
		merged, decision := MergeHandoffDeliveryObservation(current, observation)
		decisions = append(decisions, decision)
		if decision.Accepted {
			folded[key] = merged
		}
	}
	return folded, decisions
}

func HandoffDeliveryFoldKey(observation issueopscontract.IssueOpsHandoffDeliveryObservation) string {
	lifecycleID := strings.TrimSpace(observation.LifecycleID)
	lineageID := strings.TrimSpace(observation.LineageID)
	if lifecycleID == "" || lineageID == "" {
		return ""
	}
	return lifecycleID + "\x00" + lineageID
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
		ownerCheck := current
		if ownerCheck.OwnerActor == nil {
			ownerCheck.OwnerActor = next.OwnerActor
		}
		if err := validateHandoffDeliveryOwnerClaim(ownerCheck, next.OwnerClaim); err != nil {
			return current, handoffDeliveryReject(err.Error())
		}
	}
	merged := current
	merged.UpdatedAt = next.UpdatedAt
	merged.CallStaged = mergeHandoffDeliveryState(merged.CallStaged, next.CallStaged)
	merged.InputAccepted = mergeHandoffDeliveryState(merged.InputAccepted, next.InputAccepted)
	merged.NativeTurnObserved = mergeHandoffDeliveryState(merged.NativeTurnObserved, next.NativeTurnObserved)
	merged.OwnerClaimed = mergeHandoffDeliveryState(merged.OwnerClaimed, next.OwnerClaimed)
	merged.Ambiguous = mergeHandoffDeliveryState(merged.Ambiguous, next.Ambiguous)
	if merged.Request.DurableID == "" {
		merged.Request.DurableID = next.Request.DurableID
	}
	merged.Target = mergeHandoffDeliveryTarget(merged.Target, next.Target)
	merged.Timing = mergeHandoffDeliveryTiming(merged.Timing, next.Timing)
	if next.OwnerClaim.Claimed {
		merged.OwnerClaim = next.OwnerClaim
	}
	if merged.OwnerActor == nil && next.OwnerActor != nil {
		actor := *next.OwnerActor
		merged.OwnerActor = &actor
	}
	if strings.TrimSpace(next.Receipt.Location) != "" && strings.TrimSpace(next.Receipt.Digest) != "" {
		merged.Receipt = next.Receipt
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
		"launcher name": launcher.Name, "launcher version": launcher.Version, "launcher path": launcher.Path,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("delivery observation %s is required", name)
		}
		if len(value) > handoffDeliveryFieldLimit {
			return fmt.Errorf("delivery observation %s is too large", name)
		}
	}
	if launcher.Name == issueopscontract.IssueOpsHandoffDeliveryLauncherCmux {
		if launcher.RuntimeID != "" || launcher.MachineID != "" || launcher.ServerID != "" {
			return fmt.Errorf("cmux delivery observation cannot invent runtime, machine, or server identity")
		}
		return validateHandoffDeliveryEndpointIncarnation(launcher.EndpointIncarnation)
	}
	if launcher.EndpointIncarnation != nil {
		return fmt.Errorf("delivery observation endpoint incarnation is only valid for cmux")
	}
	for name, value := range map[string]string{
		"launcher runtime_id": launcher.RuntimeID, "launcher machine_id": launcher.MachineID, "launcher server_id": launcher.ServerID,
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

func validateHandoffDeliveryEndpointIncarnation(endpoint *issueopscontract.IssueOpsHandoffDeliveryEndpointIncarnation) error {
	if endpoint == nil {
		return fmt.Errorf("cmux delivery observation socket endpoint incarnation is required")
	}
	if endpoint.Kind != "unix_socket" || !filepath.IsAbs(endpoint.Path) || filepath.Clean(endpoint.Path) != endpoint.Path ||
		!filepath.IsAbs(endpoint.ParentPath) || filepath.Clean(endpoint.ParentPath) != endpoint.ParentPath ||
		filepath.Dir(endpoint.Path) != endpoint.ParentPath || endpoint.Inode == 0 || endpoint.CTimeNS <= 0 || endpoint.ParentInode == 0 {
		return fmt.Errorf("cmux delivery observation socket endpoint incarnation is invalid")
	}
	if len(endpoint.Path) > handoffDeliveryFieldLimit || len(endpoint.ParentPath) > handoffDeliveryFieldLimit {
		return fmt.Errorf("cmux delivery observation socket endpoint incarnation is too large")
	}
	if endpoint.Mode != 0o600 || endpoint.ParentMode&0o022 != 0 && endpoint.ParentMode&0o1000 == 0 {
		return fmt.Errorf("cmux delivery observation socket endpoint permissions are unsafe")
	}
	return nil
}

func validateHandoffDeliveryTarget(observation issueopscontract.IssueOpsHandoffDeliveryObservation) error {
	target := observation.Target
	if observation.Launcher.Name == issueopscontract.IssueOpsHandoffDeliveryLauncherCmux {
		if strings.TrimSpace(target.WindowID) == "" || strings.TrimSpace(target.CWD) == "" {
			return fmt.Errorf("cmux delivery observation requires exact window and cwd identity")
		}
		if observation.InputAccepted.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved &&
			(strings.TrimSpace(target.WorkspaceID) == "" || strings.TrimSpace(target.SurfaceID) == "") {
			return fmt.Errorf("cmux input receipt requires exact workspace and surface identity")
		}
	} else if strings.TrimSpace(target.TerminalID) == "" && strings.TrimSpace(target.PaneID) == "" {
		return fmt.Errorf("delivery observation terminal or pane identity is required")
	}
	for name, value := range map[string]string{
		"terminal_id":         target.TerminalID,
		"pane_id":             target.PaneID,
		"window_id":           target.WindowID,
		"workspace_id":        target.WorkspaceID,
		"surface_id":          target.SurfaceID,
		"cwd":                 target.CWD,
		"process_incarnation": target.ProcessIncarnation,
	} {
		if len(value) > handoffDeliveryFieldLimit {
			return fmt.Errorf("delivery observation %s is too large", name)
		}
	}
	if (target.PromptGeneration == nil) != (target.BaselineWorkingSequence == nil) {
		return fmt.Errorf("delivery observation prompt receipt counters are incomplete")
	}
	if target.PromptGeneration != nil && *target.PromptGeneration == 0 {
		return fmt.Errorf("delivery observation prompt generation is invalid")
	}
	if target.Process != nil {
		if len(target.Process.StartedAt) > handoffDeliveryFieldLimit || len(target.Process.Executable) > handoffDeliveryFieldLimit {
			return fmt.Errorf("delivery observation process identity is too large")
		}
		if target.Process.PID <= 0 || strings.TrimSpace(target.Process.StartedAt) == "" || strings.TrimSpace(target.Process.Executable) == "" {
			return fmt.Errorf("delivery observation process identity is invalid")
		}
	}
	return nil
}

func validateHandoffDeliveryTiming(timing *issueopscontract.IssueOpsHandoffDeliveryTiming) error {
	if timing == nil {
		return nil
	}
	const maximumTimingMS = 10 * 60 * 1000
	for name, value := range map[string]uint64{
		"preflight_ms": timing.PreflightMS, "workspace_create_ms": timing.WorkspaceCreateMS,
		"target_resolve_ms": timing.TargetResolveMS, "input_send_ms": timing.InputSendMS,
		"receiver_receipt_ms": timing.ReceiverReceiptMS,
	} {
		if value > maximumTimingMS {
			return fmt.Errorf("delivery observation %s is out of bounds", name)
		}
	}
	return nil
}

func validateHandoffDeliveryState(name string, state issueopscontract.IssueOpsHandoffDeliveryState, observation issueopscontract.IssueOpsHandoffDeliveryObservation) error {
	if len(state.Status) > handoffDeliveryFieldLimit || len(state.ObservedAt) > handoffDeliveryFieldLimit || len(state.Evidence) > handoffDeliveryFieldLimit {
		return fmt.Errorf("delivery observation %s state is too large", name)
	}
	switch state.Status {
	case issueopscontract.IssueOpsHandoffDeliveryStateNotObserved:
		if state.ObservedAt != "" || state.Evidence != "" {
			return fmt.Errorf("delivery observation %s not_observed payload must be empty", name)
		}
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
	case current.LineageID != next.LineageID:
		return "delivery observation lineage identity changed"
	case current.LifecycleID != next.LifecycleID:
		return "delivery observation lifecycle identity changed"
	case current.PromptSHA256 != next.PromptSHA256:
		return "delivery observation prompt digest changed"
	case current.MaterialSHA256 != next.MaterialSHA256:
		return "delivery observation material digest changed"
	case requestReason != "":
		return requestReason
	case current.CreatedAt != next.CreatedAt:
		return "delivery observation created timestamp changed"
	case current.SourceGeneration != next.SourceGeneration:
		return "delivery observation source generation changed"
	case !reflect.DeepEqual(current.Launcher, next.Launcher):
		return "delivery observation launcher identity changed"
	case handoffDeliveryTargetMismatch(current.Launcher.Name, current.Target, next.Target) != "":
		return handoffDeliveryTargetMismatch(current.Launcher.Name, current.Target, next.Target)
	case handoffDeliveryTimingMismatch(current.Timing, next.Timing):
		return "delivery observation timing changed"
	case current.OwnerActor != nil && next.OwnerActor != nil && !reflect.DeepEqual(current.OwnerActor, next.OwnerActor):
		return "delivery observation owner identity changed"
	default:
		return ""
	}
}

func validateHandoffDeliveryOwnerClaim(current issueopscontract.IssueOpsHandoffDeliveryObservation, claim issueopscontract.IssueOpsHandoffDeliveryOwnerClaim) error {
	generationMatches := claim.Generation == current.SourceGeneration
	if strings.HasPrefix(current.AttemptID, handoffDeliveryManualLineagePrefix+current.LifecycleID+":") &&
		strings.HasPrefix(current.LineageID, handoffDeliveryManualLineagePrefix) {
		generationMatches = claim.Generation > 1 && current.SourceGeneration == claim.Generation-1
	}
	if !claim.Claimed || !generationMatches || strings.TrimSpace(claim.ClaimedAt) == "" || current.OwnerActor == nil {
		return fmt.Errorf("delivery owner claim identity mismatch")
	}
	if !reflect.DeepEqual(claim.Actor, *current.OwnerActor) {
		return fmt.Errorf("delivery owner claim identity mismatch")
	}
	return nil
}

func handoffDeliveryRequestIdentityMismatch(current, next issueopscontract.IssueOpsHandoffDeliveryRequest) string {
	if current.DurableID != "" && next.DurableID != "" && current.DurableID != next.DurableID {
		return "delivery observation request identity changed"
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
	if len(observation.OwnerClaim.ClaimedAt) > handoffDeliveryFieldLimit {
		return fmt.Errorf("delivery owner claim timestamp is too large")
	}
	if err := validateHandoffDeliveryActorBounds(observation.OwnerClaim.Actor); err != nil {
		return err
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

func validateHandoffDeliveryRequest(request issueopscontract.IssueOpsHandoffDeliveryRequest) error {
	if len(request.DurableID) > handoffDeliveryFieldLimit {
		return fmt.Errorf("delivery observation durable request identity is invalid")
	}
	return nil
}

func validateHandoffDeliveryTimestamps(created, updated string) (time.Time, time.Time, error) {
	if len(created) > handoffDeliveryFieldLimit || len(updated) > handoffDeliveryFieldLimit {
		return time.Time{}, time.Time{}, fmt.Errorf("delivery observation timestamps are too large")
	}
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

func validateHandoffDeliveryActorBounds(actor issueopscontract.NativeActor) error {
	for name, value := range map[string]string{
		"actor host": actor.Host, "actor session_id": actor.SessionID, "actor agent_id": actor.AgentID,
	} {
		if len(value) > handoffDeliveryFieldLimit {
			return fmt.Errorf("delivery observation %s is too large", name)
		}
	}
	if actor.SessionProcess != nil && (len(actor.SessionProcess.StartedAt) > handoffDeliveryFieldLimit || len(actor.SessionProcess.Executable) > handoffDeliveryFieldLimit) {
		return fmt.Errorf("delivery observation actor process identity is too large")
	}
	for _, receipt := range actor.ProcessAncestry {
		if len(receipt.StartedAt) > handoffDeliveryFieldLimit || len(receipt.Executable) > handoffDeliveryFieldLimit {
			return fmt.Errorf("delivery observation actor process ancestry is too large")
		}
	}
	return nil
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
	case "call_staged":
		return evidence == issueopscontract.IssueOpsHandoffDeliveryEvidenceExternalCallStaged
	case "input_accepted":
		switch evidence {
		case issueopscontract.IssueOpsHandoffDeliveryEvidenceLauncherReceipt,
			issueopscontract.IssueOpsHandoffDeliveryEvidenceLauncherAccepted,
			issueopscontract.IssueOpsHandoffDeliveryEvidenceRawInput:
			return true
		case issueopscontract.IssueOpsHandoffDeliveryEvidenceOrcaDispatch:
			return observation.ExpectedOwnerHost != "omo"
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
			issueopscontract.IssueOpsHandoffDeliveryEvidenceOrcaDispatchReceipt,
			issueopscontract.IssueOpsHandoffDeliveryEvidenceTimeout,
			issueopscontract.IssueOpsHandoffDeliveryEvidenceAgentPromptStalled,
			issueopscontract.IssueOpsHandoffDeliveryEvidenceHerdrWaitState:
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
		((observation.OwnerActor != nil && observation.OwnerActor.Host == "omo") || observation.ExpectedOwnerHost == "omo")
}

func handoffDeliveryHasEvidence(observation issueopscontract.IssueOpsHandoffDeliveryObservation) bool {
	for _, state := range []issueopscontract.IssueOpsHandoffDeliveryState{
		observation.CallStaged,
		observation.InputAccepted,
		observation.NativeTurnObserved,
		observation.OwnerClaimed,
		observation.Ambiguous,
	} {
		if state.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved {
			return true
		}
	}
	return false
}

func handoffDeliveryTargetMismatch(launcher string, current, next issueopscontract.IssueOpsHandoffDeliveryTarget) string {
	if launcher == issueopscontract.IssueOpsHandoffDeliveryLauncherCmux {
		for _, values := range [][2]string{
			{current.WindowID, next.WindowID}, {current.WorkspaceID, next.WorkspaceID},
			{current.SurfaceID, next.SurfaceID}, {current.CWD, next.CWD},
		} {
			if values[0] != "" && values[1] != "" && values[0] != values[1] {
				return "delivery observation target identity changed"
			}
		}
	}
	if current.TerminalID != next.TerminalID {
		return "delivery observation process identity changed"
	}
	// PTY/runtime identity is stable, while Orca may rotate the transient pane
	// handle. A pane-only change is valid only while both observations name the
	// same nonempty PTY; current dispatch inspection still fences the assignee.
	if current.PaneID != next.PaneID && strings.TrimSpace(current.TerminalID) == "" {
		return "delivery observation process identity changed"
	}
	if current.ProcessIncarnation != "" && next.ProcessIncarnation != "" && current.ProcessIncarnation != next.ProcessIncarnation {
		return "delivery observation process identity changed"
	}
	if current.PromptGeneration != nil && next.PromptGeneration != nil && *current.PromptGeneration != *next.PromptGeneration {
		return "delivery observation process identity changed"
	}
	if current.BaselineWorkingSequence != nil && next.BaselineWorkingSequence != nil && *current.BaselineWorkingSequence != *next.BaselineWorkingSequence {
		return "delivery observation process identity changed"
	}
	if current.Process != nil && next.Process != nil && !reflect.DeepEqual(current.Process, next.Process) {
		return "delivery observation process identity changed"
	}
	return ""
}

func mergeHandoffDeliveryTarget(current, next issueopscontract.IssueOpsHandoffDeliveryTarget) issueopscontract.IssueOpsHandoffDeliveryTarget {
	if current.WindowID == "" {
		current.WindowID = next.WindowID
	}
	if current.WorkspaceID == "" {
		current.WorkspaceID = next.WorkspaceID
	}
	if current.SurfaceID == "" {
		current.SurfaceID = next.SurfaceID
	}
	if current.CWD == "" {
		current.CWD = next.CWD
	}
	if strings.TrimSpace(next.PaneID) != "" {
		current.PaneID = next.PaneID
	}
	if current.ProcessIncarnation == "" {
		current.ProcessIncarnation = next.ProcessIncarnation
	}
	if current.PromptGeneration == nil && next.PromptGeneration != nil {
		value := *next.PromptGeneration
		current.PromptGeneration = &value
	}
	if current.BaselineWorkingSequence == nil && next.BaselineWorkingSequence != nil {
		value := *next.BaselineWorkingSequence
		current.BaselineWorkingSequence = &value
	}
	if current.Process == nil && next.Process != nil {
		process := *next.Process
		current.Process = &process
	}
	return current
}

func handoffDeliveryTimingMismatch(current, next *issueopscontract.IssueOpsHandoffDeliveryTiming) bool {
	if current == nil || next == nil {
		return false
	}
	for _, values := range [][2]uint64{
		{current.PreflightMS, next.PreflightMS}, {current.WorkspaceCreateMS, next.WorkspaceCreateMS},
		{current.TargetResolveMS, next.TargetResolveMS}, {current.InputSendMS, next.InputSendMS},
		{current.ReceiverReceiptMS, next.ReceiverReceiptMS},
	} {
		if values[0] != 0 && values[1] != 0 && values[0] != values[1] {
			return true
		}
	}
	return false
}

func mergeHandoffDeliveryTiming(current, next *issueopscontract.IssueOpsHandoffDeliveryTiming) *issueopscontract.IssueOpsHandoffDeliveryTiming {
	if current == nil && next == nil {
		return nil
	}
	if current == nil {
		value := *next
		return &value
	}
	merged := *current
	if next == nil {
		return &merged
	}
	if merged.PreflightMS == 0 {
		merged.PreflightMS = next.PreflightMS
	}
	if merged.WorkspaceCreateMS == 0 {
		merged.WorkspaceCreateMS = next.WorkspaceCreateMS
	}
	if merged.TargetResolveMS == 0 {
		merged.TargetResolveMS = next.TargetResolveMS
	}
	if merged.InputSendMS == 0 {
		merged.InputSendMS = next.InputSendMS
	}
	if merged.ReceiverReceiptMS == 0 {
		merged.ReceiverReceiptMS = next.ReceiverReceiptMS
	}
	return &merged
}

func handoffDeliveryReject(reason string) issueopscontract.IssueOpsHandoffDeliveryDecision {
	return issueopscontract.IssueOpsHandoffDeliveryDecision{RejectReasons: []string{reason}}
}
