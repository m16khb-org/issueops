package issueops

import (
	"fmt"
	"issueops/internal/contract/issueops"
	"strings"
)

// ReplacementSelfHolder reports whether the requester is the session that
// holds the lease: same host (case-insensitive), session ID, and agent ID,
// and a recorded session process.
func ReplacementSelfHolder(lease issueops.WriteLease, requester issueops.NativeActor) bool {
	holder := lease.Holder
	return holder != nil && strings.EqualFold(holder.Host, requester.Host) && holder.SessionID == requester.SessionID && holder.AgentID == requester.AgentID && holder.SessionProcess != nil
}

// ValidateReplacementSelfRevoke refuses to revoke a lease whose holder
// process is still live, which is the requesting session itself. Revoking
// your own lease leaves no exit, so the error points at execution release
// instead.
func ValidateReplacementSelfRevoke(id string, generation uint64, status string) error {
	if status != "live" {
		return nil
	}
	return fmt.Errorf("revoke takes a lease away from an unresponsive holder, but this session is the live holder: "+
		"revoking your own lease leaves no exit because finalize requires the old holder to be dead. "+
		"Run `issueops execution release --id %s --generation %d` instead", strings.TrimSpace(id), generation)
}
func pidReused(receipt, observed issueops.NativeProcessReceipt) bool {
	recorded, seen := strings.TrimSpace(receipt.StartedAt), strings.TrimSpace(observed.StartedAt)
	executable := strings.TrimSpace(observed.Executable)
	return recorded != "" && seen != "" && observed.PID == receipt.PID && recorded != seen &&
		executable != "" && executable != strings.TrimSpace(receipt.Executable)
}

// ValidateReplacementCWD requires the command to run from the source root or
// the canonical worktree; the caller passes whether each matched.
func ValidateReplacementCWD(sourceMatches, workspaceMatches bool) error {
	if !sourceMatches && !workspaceMatches {
		return fmt.Errorf("execution replace cwd must be source_root or the canonical worktree")
	}
	return nil
}

// ValidateReplacementHolderProcess requires the revoking lease to still carry
// the process receipt of its old holder, which finalize needs to check
// quiescence.
func ValidateReplacementHolderProcess(holder *issueops.NativeActor) error {
	if holder == nil || holder.SessionProcess == nil {
		return fmt.Errorf("revoking lease is missing its old process receipt")
	}
	return nil
}

// ValidateReplacementQuiescence allows finalize only when the old holder is
// gone. A reused PID — another program (other executable) with another start
// time on the recorded PID — proves the recorded process no longer exists;
// before, that left the revoked lease with no exit while the PID stayed taken.
// The rendered start time alone is not proof: a TZ difference between the
// observers or a clock step can change it for a live holder (security review
// 2026-10-25), so a mismatch with the same executable, an executable-only
// mismatch (exec in place), and any unknown status stay unsafe.
func ValidateReplacementQuiescence(receipt issueops.NativeProcessReceipt, status string, observed issueops.NativeProcessReceipt) error {
	if status == "live" {
		return fmt.Errorf("old holder process is still live: pid=%d executable=%s", receipt.PID, receipt.Executable)
	}
	if status == "identity-mismatch" && pidReused(receipt, observed) {
		return nil
	}
	if status != "dead" {
		return fmt.Errorf("old holder process identity is unsafe to finalize: pid=%d status=%s", receipt.PID, status)
	}
	return nil
}
