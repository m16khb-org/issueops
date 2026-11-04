package issueops

import (
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
)

var handoffDeliveryDigest = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`^[a-f0-9]{64}$`) })

const handoffDeliveryFieldLimit = 1024

const handoffDeliveryManualLineagePrefix = "manual-direct:"

func ValidateHandoffDeliveryObservation(observation issueopscontract.HandoffDeliveryObservation) error {
	if err := validateHandoffDeliveryEnvelope(observation); err != nil {
		return err
	}
	createdAt, updatedAt, err := validateHandoffDeliveryTimestamps(observation.CreatedAt, observation.UpdatedAt)
	if err != nil {
		return err
	}
	if err := validateHandoffDeliveryComponents(observation); err != nil {
		return err
	}
	if err := validateHandoffDeliveryStates(observation, createdAt, updatedAt); err != nil {
		return err
	}
	if err := validateHandoffDeliveryConsistency(observation); err != nil {
		return err
	}
	if !handoffDeliveryHasEvidence(observation) {
		return fmt.Errorf("delivery observation requires at least one observed evidence state")
	}
	return nil
}

// validateHandoffDeliveryEnvelope checks the identifiers, request, and source generation.
func validateHandoffDeliveryEnvelope(observation issueopscontract.HandoffDeliveryObservation) error {
	if err := validateHandoffDeliveryIdentifiers(observation); err != nil {
		return err
	}
	if err := validateHandoffDeliveryRequest(observation.Request); err != nil {
		return err
	}
	if observation.SourceGeneration == 0 {
		return fmt.Errorf("delivery observation source generation is required")
	}
	return nil
}

// validateHandoffDeliveryComponents checks the receipt, launcher, target, and timing.
func validateHandoffDeliveryComponents(observation issueopscontract.HandoffDeliveryObservation) error {
	if err := validateHandoffDeliveryReceipt(observation.Receipt); err != nil {
		return err
	}
	if err := validateHandoffDeliveryLauncher(observation.Launcher); err != nil {
		return err
	}
	if err := validateHandoffDeliveryTarget(observation); err != nil {
		return err
	}
	return validateHandoffDeliveryTiming(observation.Timing)
}

// validateHandoffDeliveryConsistency checks owner identity, claim consistency, and mode evidence.
func validateHandoffDeliveryConsistency(observation issueopscontract.HandoffDeliveryObservation) error {
	if err := validateHandoffDeliveryOwnerIdentity(observation); err != nil {
		return err
	}
	if err := validateHandoffDeliveryClaimConsistency(observation); err != nil {
		return err
	}
	return validateHandoffDeliveryModeEvidence(observation)
}

// validateHandoffDeliveryIdentifiers checks the schema version, attempt/lineage/lifecycle ids and the prompt/material digests.
func validateHandoffDeliveryIdentifiers(observation issueopscontract.HandoffDeliveryObservation) error {
	if observation.SchemaVersion != issueopscontract.HandoffDeliverySchemaVersion {
		return fmt.Errorf("unsupported delivery observation schema")
	}
	if handoffDeliveryIdentifierInvalid(observation.AttemptID) {
		return fmt.Errorf("delivery observation attempt_id is required")
	}
	if handoffDeliveryIdentifierInvalid(observation.LineageID) {
		return fmt.Errorf("delivery observation lineage_id is required")
	}
	if handoffDeliveryIdentifierInvalid(observation.LifecycleID) {
		return fmt.Errorf("delivery observation lifecycle_id is required")
	}
	if !handoffDeliveryDigest().MatchString(strings.TrimSpace(observation.PromptSHA256)) {
		return fmt.Errorf("delivery observation prompt digest is invalid")
	}
	if !handoffDeliveryDigest().MatchString(strings.TrimSpace(observation.MaterialSHA256)) {
		return fmt.Errorf("delivery observation material digest is invalid")
	}
	return nil
}

func handoffDeliveryIdentifierInvalid(value string) bool {
	return strings.TrimSpace(value) == "" || len(value) > handoffDeliveryFieldLimit
}

func validateHandoffDeliveryReceipt(receipt issueopscontract.HandoffDeliveryReceipt) error {
	if strings.TrimSpace(receipt.Location) == "" || !handoffDeliveryDigest().MatchString(strings.TrimSpace(receipt.Digest)) {
		return fmt.Errorf("delivery observation receipt is invalid")
	}
	if len(receipt.Location) > handoffDeliveryFieldLimit {
		return fmt.Errorf("delivery observation receipt location is too large")
	}
	return nil
}

// validateHandoffDeliveryStates validates every delivery state in order; an unset call_staged is skipped.
func validateHandoffDeliveryStates(observation issueopscontract.HandoffDeliveryObservation, createdAt, updatedAt time.Time) error {
	for _, item := range []struct {
		name  string
		state issueopscontract.HandoffDeliveryState
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
		if err := validateHandoffDeliveryState(item.name, item.state); err != nil {
			return err
		}
		if err := validateHandoffDeliveryStateTimestamp(item.name, item.state, createdAt, updatedAt); err != nil {
			return err
		}
	}
	return nil
}

// validateHandoffDeliveryOwnerIdentity validates the owner actor bounds/shape and the expected owner host.
func validateHandoffDeliveryOwnerIdentity(observation issueopscontract.HandoffDeliveryObservation) error {
	if observation.OwnerActor != nil {
		if err := validateHandoffDeliveryActorBounds(*observation.OwnerActor); err != nil {
			return err
		}
		if err := ValidateNativeActor(*observation.OwnerActor); err != nil {
			return err
		}
	}
	switch observation.ExpectedOwnerHost {
	case "", "codex", "claude", "omo", "omp", "opencode":
		return nil
	}
	return fmt.Errorf("delivery observation expected owner host is invalid")
}

func FoldHandoffDeliveryObservations(observations []issueopscontract.HandoffDeliveryObservation) (map[string]issueopscontract.HandoffDeliveryObservation, []issueopscontract.HandoffDeliveryDecision) {
	folded := map[string]issueopscontract.HandoffDeliveryObservation{}
	decisions := []issueopscontract.HandoffDeliveryDecision{}
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
			decisions = append(decisions, issueopscontract.HandoffDeliveryDecision{Accepted: true})
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

func HandoffDeliveryFoldKey(observation issueopscontract.HandoffDeliveryObservation) string {
	lifecycleID := strings.TrimSpace(observation.LifecycleID)
	lineageID := strings.TrimSpace(observation.LineageID)
	if lifecycleID == "" || lineageID == "" {
		return ""
	}
	return lifecycleID + "\x00" + lineageID
}

func MergeHandoffDeliveryObservation(current, next issueopscontract.HandoffDeliveryObservation) (issueopscontract.HandoffDeliveryObservation, issueopscontract.HandoffDeliveryDecision) {
	if reason := handoffDeliveryMergeRejection(current, next); reason != "" {
		return current, handoffDeliveryReject(reason)
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
	return merged, issueopscontract.HandoffDeliveryDecision{Accepted: true}
}

// handoffDeliveryMergeRejection returns the first reason a merge must be rejected, or "".
func handoffDeliveryMergeRejection(current, next issueopscontract.HandoffDeliveryObservation) string {
	if err := ValidateHandoffDeliveryObservation(current); err != nil {
		return err.Error()
	}
	if err := ValidateHandoffDeliveryObservation(next); err != nil {
		return err.Error()
	}
	if reason := handoffDeliveryIdentityMismatch(current, next); reason != "" {
		return reason
	}
	currentUpdatedAt, _ := validateHandoffDeliveryTime(current.UpdatedAt)
	nextUpdatedAt, _ := validateHandoffDeliveryTime(next.UpdatedAt)
	if nextUpdatedAt.Before(currentUpdatedAt) {
		return "delivery observation update timestamp moved backward"
	}
	if current.Launcher.Name == "cmux" && next.NativeTurnObserved.Status == issueopscontract.HandoffDeliveryStateObserved &&
		next.NativeTurnObserved.Evidence == issueopscontract.HandoffDeliveryEvidenceRawInput {
		return "cmux raw input cannot prove native turn"
	}
	if next.OwnerClaimed.Status == issueopscontract.HandoffDeliveryStateObserved {
		ownerCheck := current
		if ownerCheck.OwnerActor == nil {
			ownerCheck.OwnerActor = next.OwnerActor
		}
		if err := validateHandoffDeliveryOwnerClaim(ownerCheck, next.OwnerClaim); err != nil {
			return err.Error()
		}
	}
	return ""
}

func validateHandoffDeliveryLauncher(launcher issueopscontract.HandoffDeliveryLauncher) error {
	switch launcher.Name {
	case issueopscontract.HandoffDeliveryLauncherDirect, issueopscontract.HandoffDeliveryLauncherHerdr,
		issueopscontract.HandoffDeliveryLauncherCmux:
	default:
		return fmt.Errorf("delivery observation launcher is invalid")
	}
	if err := validateHandoffDeliveryRequiredFields([]struct{ name, value string }{
		{"launcher name", launcher.Name},
		{"launcher version", launcher.Version},
		{"launcher path", launcher.Path},
	}); err != nil {
		return err
	}
	if launcher.Name == issueopscontract.HandoffDeliveryLauncherCmux {
		if launcher.RuntimeID != "" || launcher.MachineID != "" || launcher.ServerID != "" {
			return fmt.Errorf("cmux delivery observation cannot invent runtime, machine, or server identity")
		}
		return validateHandoffDeliveryEndpointIncarnation(launcher.EndpointIncarnation)
	}
	if launcher.EndpointIncarnation != nil {
		return fmt.Errorf("delivery observation endpoint incarnation is only valid for cmux")
	}
	return validateHandoffDeliveryRequiredFields([]struct{ name, value string }{
		{"launcher runtime_id", launcher.RuntimeID},
		{"launcher machine_id", launcher.MachineID},
		{"launcher server_id", launcher.ServerID},
	})
}

// validateHandoffDeliveryRequiredFields는 각 필드가 비어 있지 않고 길이 한도를 넘지 않는지 순서대로 검사한다.
func validateHandoffDeliveryRequiredFields(fields []struct{ name, value string }) error {
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("delivery observation %s is required", field.name)
		}
		if len(field.value) > handoffDeliveryFieldLimit {
			return fmt.Errorf("delivery observation %s is too large", field.name)
		}
	}
	return nil
}

func validateHandoffDeliveryEndpointIncarnation(endpoint *issueopscontract.HandoffDeliveryEndpointIncarnation) error {
	if endpoint == nil {
		return fmt.Errorf("cmux delivery observation socket endpoint incarnation is required")
	}
	if !handoffDeliveryEndpointShapeValid(endpoint) {
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

func handoffDeliveryEndpointShapeValid(endpoint *issueopscontract.HandoffDeliveryEndpointIncarnation) bool {
	cleanAbs := func(path string) bool { return filepath.IsAbs(path) && filepath.Clean(path) == path }
	return endpoint.Kind == "unix_socket" && cleanAbs(endpoint.Path) && cleanAbs(endpoint.ParentPath) &&
		filepath.Dir(endpoint.Path) == endpoint.ParentPath && endpoint.Inode != 0 && endpoint.CTimeNS > 0 && endpoint.ParentInode != 0
}

func validateHandoffDeliveryTarget(observation issueopscontract.HandoffDeliveryObservation) error {
	target := observation.Target
	if err := validateHandoffDeliveryTargetIdentity(observation, target); err != nil {
		return err
	}
	for _, field := range []struct{ name, value string }{
		{"terminal_id", target.TerminalID},
		{"pane_id", target.PaneID},
		{"window_id", target.WindowID},
		{"workspace_id", target.WorkspaceID},
		{"surface_id", target.SurfaceID},
		{"cwd", target.CWD},
		{"process_incarnation", target.ProcessIncarnation},
	} {
		if len(field.value) > handoffDeliveryFieldLimit {
			return fmt.Errorf("delivery observation %s is too large", field.name)
		}
	}
	if (target.PromptGeneration == nil) != (target.BaselineWorkingSequence == nil) {
		return fmt.Errorf("delivery observation prompt receipt counters are incomplete")
	}
	if target.PromptGeneration != nil && *target.PromptGeneration == 0 {
		return fmt.Errorf("delivery observation prompt generation is invalid")
	}
	return validateHandoffDeliveryTargetProcess(target.Process)
}

func validateHandoffDeliveryTargetIdentity(observation issueopscontract.HandoffDeliveryObservation, target issueopscontract.HandoffDeliveryTarget) error {
	if observation.Launcher.Name != issueopscontract.HandoffDeliveryLauncherCmux {
		if strings.TrimSpace(target.TerminalID) == "" && strings.TrimSpace(target.PaneID) == "" {
			return fmt.Errorf("delivery observation terminal or pane identity is required")
		}
		return nil
	}
	if strings.TrimSpace(target.WindowID) == "" || strings.TrimSpace(target.CWD) == "" {
		return fmt.Errorf("cmux delivery observation requires exact window and cwd identity")
	}
	if observation.InputAccepted.Status == issueopscontract.HandoffDeliveryStateObserved &&
		(strings.TrimSpace(target.WorkspaceID) == "" || strings.TrimSpace(target.SurfaceID) == "") {
		return fmt.Errorf("cmux input receipt requires exact workspace and surface identity")
	}
	return nil
}

func validateHandoffDeliveryTargetProcess(process *issueopscontract.NativeProcessReceipt) error {
	if process == nil {
		return nil
	}
	if len(process.StartedAt) > handoffDeliveryFieldLimit || len(process.Executable) > handoffDeliveryFieldLimit {
		return fmt.Errorf("delivery observation process identity is too large")
	}
	if process.PID <= 0 || strings.TrimSpace(process.StartedAt) == "" || strings.TrimSpace(process.Executable) == "" {
		return fmt.Errorf("delivery observation process identity is invalid")
	}
	return nil
}

func validateHandoffDeliveryTiming(timing *issueopscontract.HandoffDeliveryTiming) error {
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

func validateHandoffDeliveryState(name string, state issueopscontract.HandoffDeliveryState) error {
	if len(state.Status) > handoffDeliveryFieldLimit || len(state.ObservedAt) > handoffDeliveryFieldLimit || len(state.Evidence) > handoffDeliveryFieldLimit {
		return fmt.Errorf("delivery observation %s state is too large", name)
	}
	switch state.Status {
	case issueopscontract.HandoffDeliveryStateNotObserved:
		if state.ObservedAt != "" || state.Evidence != "" {
			return fmt.Errorf("delivery observation %s not_observed payload must be empty", name)
		}
		return nil
	case issueopscontract.HandoffDeliveryStateObserved:
		if strings.TrimSpace(state.ObservedAt) == "" || strings.TrimSpace(state.Evidence) == "" {
			return fmt.Errorf("delivery observation %s evidence is required", name)
		}
		if !validHandoffDeliveryEvidenceForState(name, state.Evidence) {
			return fmt.Errorf("delivery observation %s evidence is invalid", name)
		}
		return nil
	default:
		return fmt.Errorf("delivery observation %s state is invalid", name)
	}
}

func handoffDeliveryIdentityMismatch(current, next issueopscontract.HandoffDeliveryObservation) string {
	if reason := handoffDeliveryDigestIdentityMismatch(current, next); reason != "" {
		return reason
	}
	if reason := handoffDeliveryRequestIdentityMismatch(current.Request, next.Request); reason != "" {
		return reason
	}
	return handoffDeliveryProvenanceMismatch(current, next)
}

func handoffDeliveryDigestIdentityMismatch(current, next issueopscontract.HandoffDeliveryObservation) string {
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
	default:
		return ""
	}
}

func handoffDeliveryProvenanceMismatch(current, next issueopscontract.HandoffDeliveryObservation) string {
	if current.CreatedAt != next.CreatedAt {
		return "delivery observation created timestamp changed"
	}
	if current.SourceGeneration != next.SourceGeneration {
		return "delivery observation source generation changed"
	}
	if !reflect.DeepEqual(current.Launcher, next.Launcher) {
		return "delivery observation launcher identity changed"
	}
	if reason := handoffDeliveryTargetMismatch(current.Launcher.Name, current.Target, next.Target); reason != "" {
		return reason
	}
	if handoffDeliveryTimingMismatch(current.Timing, next.Timing) {
		return "delivery observation timing changed"
	}
	if current.OwnerActor != nil && next.OwnerActor != nil && !reflect.DeepEqual(current.OwnerActor, next.OwnerActor) {
		return "delivery observation owner identity changed"
	}
	return ""
}

func validateHandoffDeliveryOwnerClaim(current issueopscontract.HandoffDeliveryObservation, claim issueopscontract.HandoffDeliveryOwnerClaim) error {
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

func handoffDeliveryRequestIdentityMismatch(current, next issueopscontract.HandoffDeliveryRequest) string {
	if current.DurableID != "" && next.DurableID != "" && current.DurableID != next.DurableID {
		return "delivery observation request identity changed"
	}
	return ""
}

func validateHandoffDeliveryClaimConsistency(observation issueopscontract.HandoffDeliveryObservation) error {
	observed := observation.OwnerClaimed.Status == issueopscontract.HandoffDeliveryStateObserved
	claimed := observation.OwnerClaim.Claimed
	if observed != claimed {
		return fmt.Errorf("delivery owner claim state is inconsistent")
	}
	if !observed {
		return nil
	}
	if observation.OwnerClaimed.Evidence != issueopscontract.HandoffDeliveryEvidenceIssueOpsClaim {
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
	return validateHandoffDeliveryOwnerClaim(observation, observation.OwnerClaim)
}

func mergeHandoffDeliveryState(current, next issueopscontract.HandoffDeliveryState) issueopscontract.HandoffDeliveryState {
	if current.Status == issueopscontract.HandoffDeliveryStateObserved {
		return current
	}
	if next.Status == issueopscontract.HandoffDeliveryStateObserved {
		return next
	}
	return current
}

func validateHandoffDeliveryRequest(request issueopscontract.HandoffDeliveryRequest) error {
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

func validateHandoffDeliveryStateTimestamp(name string, state issueopscontract.HandoffDeliveryState, createdAt, updatedAt time.Time) error {
	if state.Status != issueopscontract.HandoffDeliveryStateObserved {
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
	for _, field := range []struct{ name, value string }{
		{"actor host", actor.Host},
		{"actor session_id", actor.SessionID},
		{"actor agent_id", actor.AgentID},
	} {
		if len(field.value) > handoffDeliveryFieldLimit {
			return fmt.Errorf("delivery observation %s is too large", field.name)
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

func validateHandoffDeliveryModeEvidence(observation issueopscontract.HandoffDeliveryObservation) error {
	if observation.Launcher.Name == issueopscontract.HandoffDeliveryLauncherHerdr &&
		observation.NativeTurnObserved.Status == issueopscontract.HandoffDeliveryStateObserved &&
		observation.NativeTurnObserved.Evidence == issueopscontract.HandoffDeliveryEvidenceHerdrWaitState {
		return fmt.Errorf("Herdr wait state cannot prove native turn") //nolint:staticcheck,nolintlint // error string starts with the proper noun Herdr
	}
	if observation.Launcher.Name == issueopscontract.HandoffDeliveryLauncherCmux &&
		observation.NativeTurnObserved.Status == issueopscontract.HandoffDeliveryStateObserved &&
		observation.NativeTurnObserved.Evidence == issueopscontract.HandoffDeliveryEvidenceRawInput {
		return fmt.Errorf("cmux raw input cannot prove native turn")
	}
	return nil
}

func validHandoffDeliveryEvidenceForState(stateName, evidence string) bool {
	switch stateName {
	case "call_staged":
		return evidence == issueopscontract.HandoffDeliveryEvidenceExternalCallStaged
	case "input_accepted":
		switch evidence {
		case issueopscontract.HandoffDeliveryEvidenceLauncherReceipt,
			issueopscontract.HandoffDeliveryEvidenceLauncherAccepted,
			issueopscontract.HandoffDeliveryEvidenceRawInput:
			return true
		}
		return false
	case "native_turn_observed":
		return evidence == issueopscontract.HandoffDeliveryEvidenceNativeReceipt
	case "owner_claimed":
		return evidence == issueopscontract.HandoffDeliveryEvidenceIssueOpsClaim
	case "ambiguous":
		switch evidence {
		case issueopscontract.HandoffDeliveryEvidenceAcceptedResponseLost,
			issueopscontract.HandoffDeliveryEvidenceTimeout,
			issueopscontract.HandoffDeliveryEvidenceAgentPromptStalled,
			issueopscontract.HandoffDeliveryEvidenceHerdrWaitState:
			return true
		}
		return false
	default:
		return false
	}
}

func handoffDeliveryHasEvidence(observation issueopscontract.HandoffDeliveryObservation) bool {
	for _, state := range []issueopscontract.HandoffDeliveryState{
		observation.CallStaged,
		observation.InputAccepted,
		observation.NativeTurnObserved,
		observation.OwnerClaimed,
		observation.Ambiguous,
	} {
		if state.Status == issueopscontract.HandoffDeliveryStateObserved {
			return true
		}
	}
	return false
}

func handoffDeliveryTargetMismatch(launcher string, current, next issueopscontract.HandoffDeliveryTarget) string {
	if launcher == issueopscontract.HandoffDeliveryLauncherCmux && handoffDeliveryCmuxTargetChanged(current, next) {
		return "delivery observation target identity changed"
	}
	if handoffDeliveryProcessIdentityChanged(current, next) {
		return "delivery observation process identity changed"
	}
	return ""
}

func handoffDeliveryCmuxTargetChanged(current, next issueopscontract.HandoffDeliveryTarget) bool {
	for _, values := range [][2]string{
		{current.WindowID, next.WindowID}, {current.WorkspaceID, next.WorkspaceID},
		{current.SurfaceID, next.SurfaceID}, {current.CWD, next.CWD},
	} {
		if values[0] != "" && values[1] != "" && values[0] != values[1] {
			return true
		}
	}
	return false
}

func handoffDeliveryProcessIdentityChanged(current, next issueopscontract.HandoffDeliveryTarget) bool {
	if current.TerminalID != next.TerminalID {
		return true
	}
	// Terminal identity is stable while the pane handle may rotate. A pane-only
	// change is valid only while both observations name the same nonempty terminal.
	if current.PaneID != next.PaneID && strings.TrimSpace(current.TerminalID) == "" {
		return true
	}
	if current.ProcessIncarnation != "" && next.ProcessIncarnation != "" && current.ProcessIncarnation != next.ProcessIncarnation {
		return true
	}
	return handoffDeliveryProcessCountersChanged(current, next)
}

func handoffDeliveryProcessCountersChanged(current, next issueopscontract.HandoffDeliveryTarget) bool {
	if current.PromptGeneration != nil && next.PromptGeneration != nil && *current.PromptGeneration != *next.PromptGeneration {
		return true
	}
	if current.BaselineWorkingSequence != nil && next.BaselineWorkingSequence != nil && *current.BaselineWorkingSequence != *next.BaselineWorkingSequence {
		return true
	}
	return current.Process != nil && next.Process != nil && !reflect.DeepEqual(current.Process, next.Process)
}

func mergeHandoffDeliveryTarget(current, next issueopscontract.HandoffDeliveryTarget) issueopscontract.HandoffDeliveryTarget {
	current = mergeHandoffDeliveryTargetIdentity(current, next)
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

// mergeHandoffDeliveryTargetIdentity는 비어 있는 문자열 식별자만 next 값으로 채운다. PaneID는 next가 비어 있지 않으면 덮어쓴다.
func mergeHandoffDeliveryTargetIdentity(current, next issueopscontract.HandoffDeliveryTarget) issueopscontract.HandoffDeliveryTarget {
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
	return current
}

func handoffDeliveryTimingMismatch(current, next *issueopscontract.HandoffDeliveryTiming) bool {
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

func mergeHandoffDeliveryTiming(current, next *issueopscontract.HandoffDeliveryTiming) *issueopscontract.HandoffDeliveryTiming {
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

func handoffDeliveryReject(reason string) issueopscontract.HandoffDeliveryDecision {
	return issueopscontract.HandoffDeliveryDecision{RejectReasons: []string{reason}}
}
