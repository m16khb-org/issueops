package issueops

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

type CleanupRemoteBranchObservation struct {
	MergeConfigured        bool
	Head                   model.CleanupRemoteBranchArtifactHead
	MergeError             error
	ArtifactSupersedeError error
	ArtifactSupersededBy   string
	IdentityError          error
	RemoteOID              string
	RemoteError            error
	TipReachedBase         bool
	TipSupersedeError      error
	TipSupersededBy        string
}

func CleanupRemoteBranchTargets(record model.IssueOpsRecord) model.CleanupRemoteBranchInventory {
	inventory := model.CleanupRemoteBranchInventory{ID: record.ID, Repo: record.Repo, Branch: strings.TrimSpace(record.Branch)}
	if record.Execution != nil {
		if branch := strings.TrimSpace(record.Execution.Workspace.Branch); branch != "" {
			inventory.Branch = branch
		}
	}
	if record.RemoteArtifact != nil {
		inventory.ArtifactURL = strings.TrimSpace(record.RemoteArtifact.URL)
	}
	return inventory
}

// A squash-merged head is safe even when Git cannot prove ancestry. Ancestry
// and a verified superseding artifact are additional paths, not replacements.
func CleanupRemoteBranchNeedsTipEvidence(facts CleanupRemoteBranchObservation) bool {
	return facts.RemoteError == nil && facts.RemoteOID != "" &&
		!(facts.MergeConfigured && facts.MergeError == nil && strings.TrimSpace(facts.Head.HeadRefOID) != "" && strings.EqualFold(strings.TrimSpace(facts.Head.HeadRefOID), facts.RemoteOID))
}

func BuildCleanupRemoteBranchPreview(record model.IssueOpsRecord, req model.CleanupRemoteBranchRequest, inventory model.CleanupRemoteBranchInventory, facts CleanupRemoteBranchObservation) (model.CleanupRemoteBranchInventory, model.CleanupRemoteBranchResult) {
	result := model.CleanupRemoteBranchResult{OK: true, ID: record.ID, Preview: !req.Apply, Branch: inventory.Branch, Missing: []string{}}
	missing := []string{}
	if inventory.Branch == "" {
		missing = append(missing, "branch_recorded")
	} else if err := ValidateBranch(inventory.Branch); err != nil {
		missing = append(missing, "branch_name_revalidated")
	}
	if inventory.Branch != "" && record.BranchPrepare != nil && inventory.Branch == strings.TrimSpace(record.BranchPrepare.BaseBranch) {
		missing = append(missing, "branch_not_base")
	}
	if record.Phase != model.IssueOpsPhaseDone {
		missing = append(missing, "phase_done")
	}
	if record.Execution != nil && record.Execution.Lease.Status != model.LeaseStatusReleased {
		missing = append(missing, "lease_released")
	}
	for _, link := range record.IssueLinks {
		if link.Type == "child" && strings.TrimSpace(link.CloseVerifiedAt) == "" {
			missing = append(missing, "child_tasks_closed")
			break
		}
	}
	if record.RemoteArtifact == nil {
		missing = append(missing, "remote_artifact_present")
	} else {
		switch {
		case !facts.MergeConfigured:
			missing = append(missing, "remote_artifact_merged")
			result.ArtifactError = "merge verification is not configured"
		case facts.MergeError != nil:
			if facts.ArtifactSupersededBy == "" {
				missing = append(missing, "remote_artifact_merged")
				result.ArtifactError = facts.MergeError.Error()
				if facts.ArtifactSupersedeError != nil {
					result.SupersedeError = facts.ArtifactSupersedeError.Error()
				}
			} else {
				inventory.SupersededBy = facts.ArtifactSupersededBy
			}
		default:
			result.ArtifactHeadBranch, result.ArtifactHeadOID = strings.TrimSpace(facts.Head.HeadRefName), strings.TrimSpace(facts.Head.HeadRefOID)
			if result.ArtifactHeadBranch == "" || result.ArtifactHeadBranch != inventory.Branch {
				missing = append(missing, "artifact_head_branch_match")
			}
		}
		if facts.IdentityError != nil {
			missing = append(missing, "remote_identity_match")
			result.RemoteIdentityError = facts.IdentityError.Error()
		}
	}
	if inventory.Branch != "" {
		if facts.RemoteError != nil {
			missing = append(missing, "remote_branch_readable")
		} else if facts.RemoteOID != "" {
			inventory.RemoteOID = facts.RemoteOID
			result.RemoteOID, result.RemoteBranchPresent = facts.RemoteOID, true
		}
	}
	if result.RemoteBranchPresent && CleanupRemoteBranchNeedsTipEvidence(facts) {
		if facts.TipReachedBase {
			result.RemoteTipReachedBase = true
		} else if facts.TipSupersededBy == "" {
			if facts.TipSupersedeError != nil {
				result.SupersedeError = facts.TipSupersedeError.Error()
			}
			missing = append(missing, "remote_tip_equals_merged_head")
		} else {
			inventory.SupersededBy = facts.TipSupersededBy
		}
	}
	result.SupersededBy = inventory.SupersededBy
	result.Missing = missing
	result.OK = len(missing) == 0
	return inventory, result
}

func ValidateCleanupRemoteBranchApply(req model.CleanupRemoteBranchRequest, fingerprint string) error {
	if !req.Confirm {
		return fmt.Errorf("cleanup remote-branch --apply requires --confirm")
	}
	if req.Fingerprint != fingerprint {
		return fmt.Errorf("stale cleanup fingerprint; run --preview again and retry with the new value")
	}
	return nil
}
