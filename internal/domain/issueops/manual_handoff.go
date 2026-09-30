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
	if record.Execution == nil || record.Execution.Mode != issueopscontract.ExecutionModeDirect || record.Execution.Lease.Status != issueopscontract.LeaseStatusReleased ||
		record.Execution.Lease.Generation != observation.SourceGeneration || record.ID != observation.LifecycleID {
		return fmt.Errorf("manual handoff delivery observation requires the exact released direct execution generation")
	}
	if !strings.HasPrefix(observation.AttemptID, handoffDeliveryManualLineagePrefix+record.ID+":") ||
		!strings.HasPrefix(observation.LineageID, handoffDeliveryManualLineagePrefix) {
		return fmt.Errorf("manual handoff delivery observation requires an isolated manual-direct namespace")
	}
	if observation.Launcher.Name != issueopscontract.IssueOpsHandoffDeliveryLauncherOrca &&
		observation.Launcher.Name != issueopscontract.IssueOpsHandoffDeliveryLauncherHerdr &&
		observation.Launcher.Name != issueopscontract.IssueOpsHandoffDeliveryLauncherCmux {
		return fmt.Errorf("manual handoff delivery observation launcher must be Orca, Herdr, or cmux")
	}
	if observation.OwnerActor != nil || observation.OwnerClaimed.Status != issueopscontract.IssueOpsHandoffDeliveryStateNotObserved ||
		observation.OwnerClaim.Claimed || observation.OwnerClaim.Generation != 0 || observation.OwnerClaim.ClaimedAt != "" ||
		observation.OwnerClaim.Actor.Host != "" || observation.OwnerClaim.Actor.SessionID != "" || observation.OwnerClaim.Actor.AgentID != "" ||
		observation.OwnerClaim.Actor.SessionProcess != nil || len(observation.OwnerClaim.Actor.ProcessAncestry) != 0 {
		return fmt.Errorf("manual handoff delivery observation cannot produce owner claim evidence")
	}
	return nil
}

func ManualCmuxHandoffIDs(request issueopscontract.ExecutionCmuxHandoffRequest) (string, string) {
	identity := strings.Join([]string{request.ID, strconv.FormatUint(request.Generation, 10), request.WindowID, request.PromptSHA256, request.MaterialSHA256}, "\x00")
	sum := sha256.Sum256([]byte(identity))
	suffix := hex.EncodeToString(sum[:12])
	attempt := handoffDeliveryManualLineagePrefix + request.ID + ":" + strconv.FormatUint(request.Generation, 10) + ":cmux:" + suffix
	lineage := strings.Join([]string{handoffDeliveryManualLineagePrefix + "generation", strconv.FormatUint(request.Generation, 10), "cmux", "window", request.WindowID, "prompt", request.PromptSHA256, "material", request.MaterialSHA256}, ":")
	return attempt, lineage
}
