package issueops

import (
	"fmt"
	"issueops/internal/contract/issueops"
	"strings"
)

type ReplacementRuntimeFacts struct {
	RuntimeID      string
	TerminalID     string
	TerminalLive   bool
	TaskLive       bool
	TaskStatus     string
	DispatchStatus string
}

func AllowReplacementRuntimeRollover(record issueops.IssueOpsRecord, processStatus string) bool {
	lease := record.Execution.Lease
	if lease.Holder == nil {
		return lease.Status == issueops.LeaseStatusReleased || lease.Status == issueops.LeaseStatusClaimable
	}
	// Adapter가 바뀐 runtime을 읽는 권한은 core가 확인한 exact holder process의
	// 종료 영수증에만 묶는다. lease 상태만으로 허용하면 live owner와 경쟁할 수 있다.
	return processStatus == "dead" &&
		(lease.Status == issueops.LeaseStatusActive || lease.Status == issueops.LeaseStatusRevoking)
}

func DeadReplacementOwnerRuntimeRollover(record issueops.IssueOpsRecord, processStatus string, inventory ReplacementRuntimeFacts) bool {
	if record.Execution == nil || record.Execution.Mode != issueops.ExecutionModeOrca || record.Execution.Orca == nil {
		return false
	}
	lease := record.Execution.Lease
	sealed := strings.TrimSpace(record.Execution.Orca.RuntimeID)
	observed := strings.TrimSpace(inventory.RuntimeID)
	return lease.Holder != nil && lease.Holder.SessionProcess != nil &&
		(lease.Status == issueops.LeaseStatusActive || lease.Status == issueops.LeaseStatusRevoking) &&
		processStatus == "dead" && observed != "" && observed != sealed &&
		inventory.TerminalID == "" && !inventory.TerminalLive
}

func ValidateReplacementRuntimeRollover(record issueops.IssueOpsRecord, processStatus string, inventory ReplacementRuntimeFacts) error {
	if record.Execution == nil || record.Execution.Mode != issueops.ExecutionModeOrca || record.Execution.Orca == nil {
		return nil
	}
	sealed := strings.TrimSpace(record.Execution.Orca.RuntimeID)
	observed := strings.TrimSpace(inventory.RuntimeID)
	if observed == "" || observed == sealed {
		return nil
	}
	if DeadReplacementOwnerRuntimeRollover(record, processStatus, inventory) {
		return nil
	}
	lease := record.Execution.Lease
	holderless := lease.Holder == nil && (lease.Status == issueops.LeaseStatusReleased || lease.Status == issueops.LeaseStatusClaimable)
	taskSettled := inventory.TaskStatus == "completed" || inventory.TaskStatus == "failed"
	dispatchSettled := inventory.DispatchStatus == "completed" || inventory.DispatchStatus == "failed" || inventory.DispatchStatus == "circuit_broken"
	if !holderless || inventory.TerminalID != "" || inventory.TerminalLive || inventory.TaskLive || !taskSettled || !dispatchSettled {
		return fmt.Errorf(
			"Orca runtime rollover owner is not quiescent: terminal_id=%s terminal_live=%t task_live=%t task_status=%s dispatch_status=%s",
			inventory.TerminalID, inventory.TerminalLive, inventory.TaskLive, inventory.TaskStatus, inventory.DispatchStatus,
		)
	}
	return nil
}

func ReplacementSelfHolder(lease issueops.WriteLease, requester issueops.NativeActor) bool {
	holder := lease.Holder
	return holder != nil && strings.EqualFold(holder.Host, requester.Host) && holder.SessionID == requester.SessionID && holder.AgentID == requester.AgentID && holder.SessionProcess != nil
}
func ValidateReplacementSelfRevoke(id string, generation uint64, status string) error {
	if status != "live" {
		return nil
	}
	return fmt.Errorf("revoke takes a lease away from an unresponsive holder, but this session is the live holder: "+
		"revoking your own lease leaves no exit because finalize requires the old holder to be dead. "+
		"Run `issueops execution release --id %s --generation %d` instead", strings.TrimSpace(id), generation)
}
func ValidateReplacementCWD(sourceMatches, workspaceMatches bool) error {
	if !sourceMatches && !workspaceMatches {
		return fmt.Errorf("execution replace cwd must be source_root or the canonical worktree")
	}
	return nil
}
func ValidateReplacementHolderProcess(holder *issueops.NativeActor) error {
	if holder == nil || holder.SessionProcess == nil {
		return fmt.Errorf("revoking lease is missing its old process receipt")
	}
	return nil
}
func ValidateReplacementQuiescence(receipt issueops.NativeProcessReceipt, status string) error {
	if status == "live" {
		return fmt.Errorf("old holder process is still live: pid=%d executable=%s", receipt.PID, receipt.Executable)
	}
	if status != "dead" {
		return fmt.Errorf("old holder process identity is unsafe to finalize: pid=%d status=%s", receipt.PID, status)
	}
	return nil
}
func ValidateReplacementOwnerQuiescence(record issueops.IssueOpsRecord, status string, facts ReplacementRuntimeFacts) error {
	if !DeadReplacementOwnerRuntimeRollover(record, status, facts) && (facts.TerminalLive || facts.TaskLive) {
		return fmt.Errorf("Orca owner is not quiescent: terminal_live=%t task_live=%t task_status=%s dispatch_status=%s", facts.TerminalLive, facts.TaskLive, facts.TaskStatus, facts.DispatchStatus)
	}
	return nil
}
