package issueops

import (
	"fmt"
	"issueops/internal/contract/issueops"
	"strings"
)

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
