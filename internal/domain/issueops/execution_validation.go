package issueops

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	issueopscontract "issueops/internal/contract/issueops"
)

func ValidateExecution(execution issueopscontract.Execution) error {
	if err := validateExecutionCore(execution); err != nil {
		return err
	}
	if err := validateExecutionPending(execution.Pending); err != nil {
		return err
	}
	if err := validateExecutionCompletions(execution); err != nil {
		return err
	}
	if execution.Failure != nil {
		if execution.Failure.Code == "" || execution.Failure.At == "" || len(execution.Failure.Message) > 4096 {
			return fmt.Errorf("execution failure is invalid")
		}
	}
	if execution.SyncBaseResolution != nil {
		if err := validateExecutionSyncBaseResolution(execution, *execution.SyncBaseResolution); err != nil {
			return err
		}
	}
	for _, event := range execution.SyncBaseEvents {
		if err := validateExecutionSyncBaseEvent(event); err != nil {
			return err
		}
	}
	return nil
}

func validateExecutionCore(execution issueopscontract.Execution) error {
	if err := validateWorkspace(execution.Workspace); err != nil {
		return err
	}
	return validateWriteLease(execution.Lease)
}

func validateExecutionPending(pending *issueopscontract.ExternalIntent) error {
	if pending == nil {
		return nil
	}
	if pending.OperationID == "" || pending.Kind == "" || pending.Marker == "" || pending.StartedAt == "" {
		return fmt.Errorf("pending external intent is incomplete")
	}
	return nil
}

func validateExecutionCompletions(execution issueopscontract.Execution) error {
	if execution.Completion != nil {
		if err := validateExecutionCompletion(*execution.Completion); err != nil {
			return err
		}
		if execution.Completion.Generation > execution.Lease.Generation {
			return fmt.Errorf("execution completion generation exceeds the lease generation")
		}
	}
	for _, entry := range execution.CompletionHistory {
		if entry.Generation == 0 || strings.TrimSpace(entry.Reason) == "" || strings.TrimSpace(entry.ReopenedAt) == "" {
			return fmt.Errorf("execution completion history entry is incomplete")
		}
		if entry.Generation >= execution.Lease.Generation {
			return fmt.Errorf("execution completion history generation must precede the lease generation")
		}
		if err := validateExecutionCompletion(entry.Completion); err != nil {
			return fmt.Errorf("execution completion history: %w", err)
		}
		if entry.Completion.Generation != entry.Generation {
			return fmt.Errorf("execution completion history generation conflicts with its completion")
		}
	}
	return nil
}

func validateExecutionSyncBaseResolution(execution issueopscontract.Execution, resolution issueopscontract.ExecutionSyncBaseResolution) error {
	if !syncBaseResolutionBindsCompletion(execution, resolution) {
		return fmt.Errorf("execution sync-base resolution must bind the released current completion")
	}
	if !validCommitSHA(resolution.BaseOID) || strings.TrimSpace(resolution.StartedAt) == "" || len(resolution.ConflictFiles) == 0 {
		return fmt.Errorf("execution sync-base resolution is incomplete")
	}
	if err := ValidateNativeActor(resolution.Actor); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, path := range resolution.ConflictFiles {
		if !validSyncBaseConflictPath(path) || seen[filepath.Clean(path)] {
			return fmt.Errorf("execution sync-base resolution conflict path is invalid")
		}
		seen[filepath.Clean(path)] = true
	}
	return nil
}

func syncBaseResolutionBindsCompletion(execution issueopscontract.Execution, resolution issueopscontract.ExecutionSyncBaseResolution) bool {
	return execution.Lease.Status == issueopscontract.LeaseStatusReleased && execution.Completion != nil &&
		resolution.Generation != 0 && resolution.Generation == execution.Lease.Generation &&
		resolution.CompletionGeneration != 0 && resolution.CompletionGeneration == execution.Completion.Generation
}

// validSyncBaseConflictPath는 정규화된 상대 경로이면서 작업 트리 밖으로 나가지 않는지 확인한다.
func validSyncBaseConflictPath(path string) bool {
	clean := filepath.Clean(path)
	return path != "" && path == clean && !filepath.IsAbs(path) && clean != "." && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func validateExecutionCompletion(completion issueopscontract.ExecutionCompletion) error {
	if completion.Generation == 0 || !validCommitSHA(completion.FinalHead) || strings.TrimSpace(completion.VerificationReportPath) == "" ||
		len(completion.Verification) == 0 || strings.TrimSpace(completion.RemoteArtifactURL) == "" || strings.TrimSpace(completion.CompletedAt) == "" {
		return fmt.Errorf("execution completion is incomplete")
	}
	for _, evidence := range completion.Verification {
		if strings.TrimSpace(evidence) == "" {
			return fmt.Errorf("execution completion verification must be nonempty")
		}
	}
	return nil
}

func validateExecutionSyncBaseEvent(event issueopscontract.ExecutionSyncBaseEvent) error {
	if event.Mode != issueopscontract.ExecutionSyncBaseEventApply && event.Mode != issueopscontract.ExecutionSyncBaseEventFinalize {
		return fmt.Errorf("execution sync-base event mode must be apply or finalize")
	}
	if !validCommitSHA(event.BaseOID) || !validCommitSHA(event.MergeCommit) {
		return fmt.Errorf("execution sync-base event requires full base and merge commit OIDs")
	}
	if strings.TrimSpace(event.BaseBranch) == "" || strings.TrimSpace(event.Actor) == "" || strings.TrimSpace(event.At) == "" {
		return fmt.Errorf("execution sync-base event is incomplete")
	}
	if event.ConflictFiles < 0 {
		return fmt.Errorf("execution sync-base event conflict count must not be negative")
	}
	return nil
}

func validateWorkspace(workspace issueopscontract.Workspace) error {
	for _, field := range []struct{ name, value string }{
		{"source_root", workspace.SourceRoot},
		{"root", workspace.Root},
		{"branch", workspace.Branch},
		{"base_head", workspace.BaseHead},
		{"linked_at", workspace.LinkedAt},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("workspace %s is required", field.name)
		}
	}
	if workspace.SourceRoot == workspace.Root {
		return fmt.Errorf("canonical worktree must be isolated from source_root")
	}
	return nil
}

func validateWriteLease(lease issueopscontract.WriteLease) error {
	if lease.Generation == 0 {
		return fmt.Errorf("lease generation must start at 1")
	}
	switch lease.Status {
	case issueopscontract.LeaseStatusClaimable:
		if lease.Holder != nil || !validSHA256(lease.ClaimTokenSHA256) {
			return fmt.Errorf("claimable lease requires no holder and one token hash")
		}
	case issueopscontract.LeaseStatusActive:
		if lease.Holder == nil || lease.ClaimTokenSHA256 != "" || lease.ClaimedAt == "" {
			return fmt.Errorf("active lease requires one holder and no token hash")
		}
		return ValidateNativeActor(*lease.Holder)
	case issueopscontract.LeaseStatusRevoking:
		// 이전 holder는 quiescence 진단에만 남고 이 상태에서는 writer가 아니다.
		if lease.Holder == nil || lease.ClaimTokenSHA256 != "" {
			return fmt.Errorf("revoking lease requires the fenced holder and no token hash")
		}
		return ValidateNativeActor(*lease.Holder)
	case issueopscontract.LeaseStatusReleased:
		if lease.Holder != nil || lease.ClaimTokenSHA256 != "" {
			return fmt.Errorf("released lease must not retain a holder or token hash")
		}
	default:
		return fmt.Errorf("unsupported lease status %q", lease.Status)
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validCommitSHA(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
