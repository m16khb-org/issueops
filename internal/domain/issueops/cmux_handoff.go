package issueops

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	issueopscontract "issueops/internal/contract/issueops"
)

// CmuxResult builds the cmux handoff result from the delivery observation. OK
// requires the input to be accepted without an ambiguous delivery; the other
// delivery stages are reported as booleans. status and recovery are the
// caller-chosen status and recovery artifact directory.
func CmuxResult(request issueopscontract.ExecutionCmuxHandoffRequest, observation issueopscontract.HandoffDeliveryObservation, status, recovery string) issueopscontract.ExecutionCmuxHandoffResult {
	return issueopscontract.ExecutionCmuxHandoffResult{
		OK: observation.InputAccepted.Status == issueopscontract.HandoffDeliveryStateObserved &&
			observation.Ambiguous.Status != issueopscontract.HandoffDeliveryStateObserved,
		ID: request.ID, Generation: request.Generation, Status: status,
		Launcher: observation.Launcher, Target: observation.Target, Timing: observation.Timing,
		InputAccepted:       observation.InputAccepted.Status == issueopscontract.HandoffDeliveryStateObserved,
		NativeTurnObserved:  observation.NativeTurnObserved.Status == issueopscontract.HandoffDeliveryStateObserved,
		OwnerClaimed:        observation.OwnerClaimed.Status == issueopscontract.HandoffDeliveryStateObserved,
		RecoveryArtifactDir: recovery, ObservationReceipt: observation.Receipt,
	}
}

// CmuxObserved returns a delivery state marked observed at the given time
// with its evidence.
func CmuxObserved(at, evidence string) issueopscontract.HandoffDeliveryState {
	return issueopscontract.HandoffDeliveryState{Status: issueopscontract.HandoffDeliveryStateObserved, ObservedAt: at, Evidence: evidence}
}

// CmuxNotObserved returns a delivery state marked not observed.
func CmuxNotObserved() issueopscontract.HandoffDeliveryState {
	return issueopscontract.HandoffDeliveryState{Status: issueopscontract.HandoffDeliveryStateNotObserved}
}

// ValidCmuxDigest reports whether value is a 64-character lowercase
// hexadecimal SHA-256 digest.
func ValidCmuxDigest(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

// CmuxRequestContainsNUL reports whether any string field of the cmux handoff
// request contains a NUL byte, which cannot pass through process arguments or
// a socket command.
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

// CmuxCanonicalPromptPath maps an absolute, clean prompt file path under
// canonicalRoot or one of rootAliases to the same relative path under
// canonicalRoot. It fails when the path is not clean and absolute or lies
// outside every root.
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

// ValidateCmuxReleasedRecord requires the record to hold a released execution
// lease at exactly the given generation before a cmux handoff may start.
func ValidateCmuxReleasedRecord(record issueopscontract.Record, generation uint64) error {
	if record.Execution == nil ||
		record.Execution.Lease.Status != issueopscontract.LeaseStatusReleased || record.Execution.Lease.Generation != generation {
		return fmt.Errorf("cmux handoff requires the exact released direct execution generation")
	}
	return nil
}

// ValidateCmuxAttempt rejects a cmux handoff when the observations already
// contain the same lineage, or a staged cmux call for the same lifecycle and
// generation. A failed handoff must be inspected, never retried.
func ValidateCmuxAttempt(request issueopscontract.ExecutionCmuxHandoffRequest, observations []issueopscontract.HandoffDeliveryObservation) error {
	_, lineageID := ManualCmuxHandoffIDs(request)
	for _, observation := range observations {
		if observation.LifecycleID == request.ID && observation.LineageID == lineageID {
			return fmt.Errorf("cmux handoff attempt already exists; inspect the existing lineage and do not retry")
		}
		if observation.LifecycleID == request.ID && observation.SourceGeneration == request.Generation &&
			observation.Launcher.Name == issueopscontract.HandoffDeliveryLauncherCmux &&
			observation.CallStaged.Status == issueopscontract.HandoffDeliveryStateObserved {
			return fmt.Errorf("cmux handoff generation already has a staged attempt; inspect the existing lineage and do not retry")
		}
	}
	return nil
}
