package issueops

import (
	"fmt"
	model "issueops/internal/contract/issueops"
	"strings"
)

// ValidateReplacementGeneration checks the expected lease generation of a
// replace command against the record. An unprepared execution or a stale
// generation fails; generation 0 is accepted only in discovery mode.
func ValidateReplacementGeneration(record model.Record, generation uint64, discovery bool) error {
	if record.Execution == nil {
		return fmt.Errorf("IssueOps execution v1 is not prepared")
	}
	if (!discovery && generation == 0) || (generation != 0 && record.Execution.Lease.Generation != generation) {
		return fmt.Errorf("stale lease generation: current=%d expected=%d", record.Execution.Lease.Generation, generation)
	}
	return nil
}

// ValidateReplacementPreview allows a replace preview only from an active,
// released, or claimable lease.
func ValidateReplacementPreview(status model.LeaseStatus) error {
	if status != model.LeaseStatusActive && status != model.LeaseStatusReleased && status != model.LeaseStatusClaimable {
		return fmt.Errorf("replace preview is unavailable from %s", status)
	}
	return nil
}

// ValidateReplacementRevoke requires an active lease and a non-blank reason
// before a revoke.
func ValidateReplacementRevoke(lease model.WriteLease, reason string) error {
	if lease.Status != model.LeaseStatusActive || strings.TrimSpace(reason) == "" {
		return fmt.Errorf("revoke requires an active lease and a reason")
	}
	return nil
}

// RevokeReplacement returns lease fenced for replacement: the generation
// increases, the status becomes revoking, and the time and trimmed reason are
// recorded. The old holder is kept for the quiescence check.
func RevokeReplacement(lease model.WriteLease, reason, now string) model.WriteLease {
	lease.Generation++
	lease.Status = model.LeaseStatusRevoking
	lease.ReplacedAt = now
	lease.ReplacementReason = strings.TrimSpace(reason)
	return lease
}

// FinalizeReplacement 함수는 revoke된 lease를 claim 가능 상태로 넘긴다. holder를 비우고 claim
// token 해시를 기록한다. 없는 workspace에는 claim token을 줄 수 없으므로 released로 두어, 유령
// owner를 만들지 않고도 abandon이나 새 준비가 가능하게 한다.
func FinalizeReplacement(lease model.WriteLease, workspaceAbsent bool, tokenSHA string) model.WriteLease {
	lease.Holder = nil
	lease.ClaimTokenSHA256 = tokenSHA
	lease.Status = model.LeaseStatusClaimable
	if workspaceAbsent {
		lease.Status = model.LeaseStatusReleased
		lease.ClaimTokenSHA256 = ""
	}
	return lease
}

// CompletedReplacementBase reports whether a replacement preview starts from
// a completed generation: the lease is released or claimable and a completion
// is stamped. It fails when the stamped generation is invalid, conflicts with
// the selected completion generation, or the prepared base branch is missing.
func CompletedReplacementBase(record model.Record, selected uint64) (bool, error) {
	execution := record.Execution
	if execution == nil || execution.Completion == nil || (execution.Lease.Status != model.LeaseStatusReleased && execution.Lease.Status != model.LeaseStatusClaimable) {
		return false, nil
	}
	generation := execution.Completion.Generation
	if generation == 0 {
		return false, fmt.Errorf("invalid or missing stamped completion generation")
	}
	if selected != 0 && selected != generation {
		return false, fmt.Errorf("completion_generation conflicts with stamped completion generation %d", generation)
	}
	if record.BranchPrepare == nil || strings.TrimSpace(record.BranchPrepare.BaseBranch) == "" {
		return false, fmt.Errorf("completed replacement preview requires branch_prepare.base_branch")
	}
	return true, nil
}
