package issueops

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	issueopscontract "issueops/internal/contract/issueops"
)

func CmuxResult(request issueopscontract.ExecutionCmuxHandoffRequest, observation issueopscontract.IssueOpsHandoffDeliveryObservation, status, recovery string) issueopscontract.ExecutionCmuxHandoffResult {
	return issueopscontract.ExecutionCmuxHandoffResult{
		OK: observation.InputAccepted.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved &&
			observation.Ambiguous.Status != issueopscontract.IssueOpsHandoffDeliveryStateObserved,
		ID: request.ID, Generation: request.Generation, Status: status,
		Launcher: observation.Launcher, Target: observation.Target, Timing: observation.Timing,
		InputAccepted:       observation.InputAccepted.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved,
		NativeTurnObserved:  observation.NativeTurnObserved.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved,
		OwnerClaimed:        observation.OwnerClaimed.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved,
		RecoveryArtifactDir: recovery, ObservationReceipt: observation.Receipt,
	}
}

func CmuxObserved(at, evidence string) issueopscontract.IssueOpsHandoffDeliveryState {
	return issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateObserved, ObservedAt: at, Evidence: evidence}
}

func CmuxNotObserved() issueopscontract.IssueOpsHandoffDeliveryState {
	return issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateNotObserved}
}

func ValidCmuxDigest(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func CmuxRequestContainsNUL(request issueopscontract.ExecutionCmuxHandoffRequest) bool {
	for _, value := range []string{
		request.ID, request.CmuxExecutable, request.CmuxVersion, request.CmuxBuild, request.SocketPath,
		request.WindowID, request.CWD, request.Host, request.HostExecutable, request.Model, request.Effort,
		request.PromptFile, request.PromptSHA256, request.MaterialSHA256,
	} {
		if strings.ContainsRune(value, 0) {
			return true
		}
	}
	return false
}

func CmuxCanonicalPromptPath(canonicalRoot, promptPath string, rootAliases ...string) (string, error) {
	if !filepath.IsAbs(promptPath) || filepath.Clean(promptPath) != promptPath {
		return "", fmt.Errorf("cmux prompt file must be a clean path inside the canonical worktree")
	}
	for _, base := range append([]string{canonicalRoot}, rootAliases...) {
		if !filepath.IsAbs(base) || filepath.Clean(base) != base {
			continue
		}
		relative, err := filepath.Rel(base, promptPath)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		return filepath.Join(canonicalRoot, relative), nil
	}
	return "", fmt.Errorf("cmux prompt file must be inside the canonical worktree")
}

func ValidateCmuxReleasedRecord(record issueopscontract.IssueOpsRecord, generation uint64) error {
	if record.Execution == nil || record.Execution.Mode != issueopscontract.ExecutionModeDirect ||
		record.Execution.Lease.Status != issueopscontract.LeaseStatusReleased || record.Execution.Lease.Generation != generation {
		return fmt.Errorf("cmux handoff requires the exact released direct execution generation")
	}
	return nil
}
func ValidateCmuxAttempt(request issueopscontract.ExecutionCmuxHandoffRequest, observations []issueopscontract.IssueOpsHandoffDeliveryObservation) error {
	_, lineageID := ManualCmuxHandoffIDs(request)
	for _, observation := range observations {
		if observation.LifecycleID == request.ID && observation.LineageID == lineageID {
			return fmt.Errorf("cmux handoff attempt already exists; inspect the existing lineage and do not retry")
		}
		if observation.LifecycleID == request.ID && observation.SourceGeneration == request.Generation &&
			observation.Launcher.Name == issueopscontract.IssueOpsHandoffDeliveryLauncherCmux &&
			observation.CallStaged.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved {
			return fmt.Errorf("cmux handoff generation already has a staged attempt; inspect the existing lineage and do not retry")
		}
	}
	return nil
}
