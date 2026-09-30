package issueops

import (
	"fmt"
	model "issueops/internal/contract/issueops"
	"strings"
)

func ValidateReplacementGeneration(record model.IssueOpsRecord, generation uint64, discovery bool) error {
	if record.Execution == nil {
		return fmt.Errorf("IssueOps execution v1 is not prepared")
	}
	if (!discovery && generation == 0) || (generation != 0 && record.Execution.Lease.Generation != generation) {
		return fmt.Errorf("stale lease generation: current=%d expected=%d", record.Execution.Lease.Generation, generation)
	}
	return nil
}
func ValidateReplacementPreview(status model.LeaseStatus) error {
	if status != model.LeaseStatusActive && status != model.LeaseStatusReleased && status != model.LeaseStatusClaimable {
		return fmt.Errorf("replace preview is unavailable from %s", status)
	}
	return nil
}
func ValidateReplacementRevoke(lease model.WriteLease, reason string) error {
	if lease.Status != model.LeaseStatusActive || strings.TrimSpace(reason) == "" {
		return fmt.Errorf("revoke requires an active lease and a reason")
	}
	return nil
}
func RevokeReplacement(lease model.WriteLease, reason, now string) model.WriteLease {
	lease.Generation++
	lease.Status = model.LeaseStatusRevoking
	lease.ReplacedAt = now
	lease.ReplacementReason = strings.TrimSpace(reason)
	return lease
}

// An absent workspace cannot receive a claim token. Release it so abandonment
// or fresh preparation remains possible without inventing a claimable owner.
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
func SealReplacementOwner(binding *model.OrcaBinding, generation uint64, artifacts model.ReplacementArtifacts) {
	if binding == nil {
		return
	}
	binding.LeaseGeneration = generation
	binding.ArtifactIdentityVersion = model.OrcaArtifactIdentityVersion
	binding.IssueBodySHA256 = artifacts.IssueBodySHA256
	binding.ContextPacketSHA256 = artifacts.ContextPacketSHA256
	binding.OwnerPromptSHA256 = artifacts.OwnerPromptSHA256
}
func CompletedReplacementBase(record model.IssueOpsRecord, selected uint64) (bool, error) {
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
