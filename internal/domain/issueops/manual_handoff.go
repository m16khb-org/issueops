package issueops

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	issueopscontract "issueops/internal/contract/issueops"
)

func ValidateManualHandoffDeliveryObservation(record issueopscontract.IssueOpsRecord, observation issueopscontract.IssueOpsHandoffDeliveryObservation) error {
	if !manualHandoffReleasedDirectGeneration(record, observation) {
		return fmt.Errorf("manual handoff delivery observation requires the exact released direct execution generation")
	}
	if !strings.HasPrefix(observation.AttemptID, handoffDeliveryManualLineagePrefix+record.ID+":") ||
		!strings.HasPrefix(observation.LineageID, handoffDeliveryManualLineagePrefix) {
		return fmt.Errorf("manual handoff delivery observation requires an isolated manual-direct namespace")
	}
	if observation.Launcher.Name != issueopscontract.IssueOpsHandoffDeliveryLauncherHerdr &&
		observation.Launcher.Name != issueopscontract.IssueOpsHandoffDeliveryLauncherCmux {
		return fmt.Errorf("manual handoff delivery observation launcher must be Herdr or cmux")
	}
	if manualHandoffHasOwnerClaimEvidence(observation) {
		return fmt.Errorf("manual handoff delivery observation cannot produce owner claim evidence")
	}
	return nil
}

func manualHandoffReleasedDirectGeneration(record issueopscontract.IssueOpsRecord, observation issueopscontract.IssueOpsHandoffDeliveryObservation) bool {
	return record.Execution != nil && record.Execution.Lease.Status == issueopscontract.LeaseStatusReleased &&
		record.Execution.Lease.Generation == observation.SourceGeneration && record.ID == observation.LifecycleID
}

func manualHandoffHasOwnerClaimEvidence(observation issueopscontract.IssueOpsHandoffDeliveryObservation) bool {
	claim := observation.OwnerClaim
	return observation.OwnerActor != nil || observation.OwnerClaimed.Status != issueopscontract.IssueOpsHandoffDeliveryStateNotObserved ||
		claim.Claimed || claim.Generation != 0 || claim.ClaimedAt != "" ||
		claim.Actor.Host != "" || claim.Actor.SessionID != "" || claim.Actor.AgentID != "" ||
		claim.Actor.SessionProcess != nil || len(claim.Actor.ProcessAncestry) != 0
}

func ManualCmuxHandoffIDs(request issueopscontract.ExecutionCmuxHandoffRequest) (string, string) {
	identity := strings.Join([]string{request.ID, strconv.FormatUint(request.Generation, 10), request.WindowID, request.PromptSHA256, request.MaterialSHA256}, "\x00")
	sum := sha256.Sum256([]byte(identity))
	suffix := hex.EncodeToString(sum[:12])
	attempt := handoffDeliveryManualLineagePrefix + request.ID + ":" + strconv.FormatUint(request.Generation, 10) + ":cmux:" + suffix
	lineage := strings.Join([]string{handoffDeliveryManualLineagePrefix + "generation", strconv.FormatUint(request.Generation, 10), "cmux", "window", request.WindowID, "prompt", request.PromptSHA256, "material", request.MaterialSHA256}, ":")
	return attempt, lineage
}
