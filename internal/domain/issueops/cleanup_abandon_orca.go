package issueops

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

func ValidateCleanupAbandonPending(record model.IssueOpsRecord, worktreePresent, localKind bool) error {
	if !localKind {
		return fmt.Errorf("pending external intent kind %q is not local-only; run `issueops execution reconcile --id %s --preview --json` until it settles, then reclaim the Orca worktree with `orca worktree remove` before retrying abandon", record.Execution.Pending.Kind, record.ID)
	}
	if record.Execution.Mode != model.ExecutionModeOrca {
		return fmt.Errorf("Orca intent requires an Orca execution record")
	}
	if worktreePresent {
		return fmt.Errorf("recorded workspace root still exists on disk")
	}
	return nil
}

type CleanupAbandonIntentIdentity struct {
	LifecycleID, Marker, PendingKind string
	Generation                       uint64
}

func ValidateCleanupAbandonIntent(record model.IssueOpsRecord, intent CleanupAbandonIntentIdentity) error {
	pending := record.Execution.Pending
	if intent.LifecycleID != record.ID || intent.Marker != pending.Marker || intent.Generation != record.Execution.Lease.Generation || pending.Kind != intent.PendingKind {
		return fmt.Errorf("Orca external intent row does not belong to this lifecycle")
	}
	return nil
}

func ValidateCleanupAbandonIntentObservation(stage string, candidates int, authoritativeZero bool, err error) error {
	if err != nil {
		return fmt.Errorf("Orca %s intent inventory is ambiguous; intent retained: %w", stage, err)
	}
	if candidates != 0 {
		return fmt.Errorf("Orca %s intent inventory found %d candidate(s); the mutation may have landed", stage, candidates)
	}
	if !authoritativeZero {
		return fmt.Errorf("Orca %s intent inventory returned a non-authoritative zero", stage)
	}
	return nil
}

func CleanupAbandonPendingRecovery(id string, cause error) string {
	detail := cause.Error()
	if strings.Contains(detail, "execution reconcile") && strings.Contains(detail, "worktree") {
		return detail
	}
	return fmt.Sprintf("%s; run `issueops execution reconcile --id %s --preview --json` until it settles, then reclaim the Orca worktree with `orca worktree remove` before retrying abandon", detail, id)
}

func CleanupAbandonOrcaTarget(record model.IssueOpsRecord) *model.OrcaBinding {
	if record.Execution == nil || record.Execution.Mode != model.ExecutionModeOrca || record.Execution.Orca == nil || strings.TrimSpace(record.Execution.Orca.TaskID) == "" {
		return nil
	}
	return record.Execution.Orca
}

func CleanupAbandonAllowsRuntimeRollover(record model.IssueOpsRecord, holderless bool) bool {
	return holderless && record.Execution.Lease.Holder == nil
}

type CleanupAbandonOrcaObservation struct {
	InspectorAvailable, TaskLive, TerminalLive, TerminalsReachable bool
	TaskStatus, DispatchStatus                                     string
	Err                                                            error
}

func ValidateCleanupAbandonOrcaObservation(facts CleanupAbandonOrcaObservation) error {
	if !facts.InspectorAvailable {
		return fmt.Errorf("Orca owner inspector is not configured; resolve this cycle with `issueops cleanup finish` or `issueops cleanup orphan`")
	}
	if facts.Err != nil {
		return fmt.Errorf("Orca owner inventory is ambiguous; resolve this cycle with `issueops cleanup finish` or `issueops cleanup orphan`: %w", facts.Err)
	}
	if facts.TaskLive || (facts.TerminalLive && !facts.TerminalsReachable) {
		return fmt.Errorf("Orca resources are still live (task_status=%q dispatch_status=%q terminal_live=%t terminal_reachable_by_stop=%t); abandon leaves them without an owner, so use `issueops cleanup finish` or `issueops cleanup orphan`", facts.TaskStatus, facts.DispatchStatus, facts.TerminalLive, facts.TerminalsReachable)
	}
	return nil
}
