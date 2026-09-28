package issueops

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

type CleanupFinishObservation struct {
	SupersedeError                                            string
	DefaultBranch                                             string
	PreparedBasePresent, BaseObserved                         bool
	WorktreeIdentityConflict, WorktreeUnobservable, CWDWithin bool
	WorkspaceMissing                                          []string
	Occupants                                                 []model.CleanupWorkspaceProcess
	WorktreeClean, BranchObservable                           bool
	RemoteReadable                                            bool
	RemoteOID                                                 string
}

func CleanupFinishTargets(record model.IssueOpsRecord) model.CleanupFinishInventory {
	inventory := model.CleanupFinishInventory{ID: record.ID, Repo: record.Repo, Branch: strings.TrimSpace(record.Branch)}
	if record.RemoteArtifact != nil {
		inventory.RemoteURL = record.RemoteArtifact.URL
	}
	if record.Execution != nil {
		inventory.WorktreeRoot = strings.TrimSpace(record.Execution.Workspace.Root)
		if branch := strings.TrimSpace(record.Execution.Workspace.Branch); branch != "" {
			inventory.Branch = branch
		}
		if record.Execution.Orca != nil {
			inventory.OrcaWorktreeID = record.Execution.Orca.WorktreeID
		}
	}
	if inventory.WorktreeRoot == "" {
		inventory.WorktreeRoot = strings.TrimSpace(record.WorktreePath)
	}
	return inventory
}

func BuildCleanupFinishPreview(record model.IssueOpsRecord, req model.CleanupFinishRequest, inventory model.CleanupFinishInventory, observed CleanupFinishObservation) model.CleanupFinishResult {
	result := model.CleanupFinishResult{
		OK: true, ID: record.ID, Preview: !req.Apply, Missing: []string{},
		WorktreePath: inventory.WorktreeRoot, Branch: inventory.Branch,
		WorktreePresent: inventory.WorktreePresent, BranchPresent: inventory.BranchOID != "",
		OrcaWorktreeID: inventory.OrcaWorktreeID, WorkspaceProcesses: observed.Occupants,
		OrcaTerminals: inventory.OrcaTerminals, SupersededBy: inventory.SupersededBy, SupersedeError: observed.SupersedeError,
	}
	missing := result.Missing
	if record.Phase != model.IssueOpsPhaseDone {
		missing = append(missing, "phase_done")
	}
	if record.Execution != nil && record.Execution.Lease.Status != model.LeaseStatusReleased {
		missing = append(missing, "lease_released")
	}
	if !req.Merged && (observed.SupersedeError != "" || inventory.SupersededBy == "") {
		missing = append(missing, "remote_artifact_merged")
	}
	if !req.CompletionReflected {
		missing = append(missing, "completion_reflected")
	}
	if !req.IssueClosed {
		missing = append(missing, "issue_closed")
	}
	if prepared := PreparedBaseBranch(record); prepared != "" {
		base := strings.TrimSpace(req.MergedBaseBranch)
		switch {
		case base == "":
			missing = append(missing, "merged_base_branch_unobserved")
		case inventory.SupersededBy != "":
		default:
			if slugs := ClassifyCleanupMergedBase(prepared, base, observed.DefaultBranch, observed.PreparedBasePresent, observed.BaseObserved); len(slugs) > 0 {
				missing = append(missing, slugs...)
			} else if base != prepared {
				result.RetargetedBase = &model.CleanupRetargetedBase{PreparedBase: prepared, ObservedBase: base, DefaultBranch: observed.DefaultBranch, PreparedBaseRemoteAbsent: true}
			}
		}
	}
	for _, link := range record.IssueLinks {
		if link.Type == "child" && strings.TrimSpace(link.CloseVerifiedAt) == "" {
			missing = append(missing, "child_tasks_closed")
			break
		}
	}
	if observed.WorktreeIdentityConflict {
		missing = append(missing, "worktree_identity_conflict")
	}
	if observed.WorktreeUnobservable {
		missing = append(missing, "worktree_observable")
	}
	if inventory.WorktreePresent {
		if strings.TrimSpace(req.CWD) == "" {
			missing = append(missing, "cwd_unresolved")
		} else if observed.CWDWithin {
			missing = append(missing, "cwd_outside_worktree")
		}
		missing = append(missing, observed.WorkspaceMissing...)
		if !observed.WorktreeClean {
			missing = append(missing, "worktree_clean")
		}
	}
	if inventory.Branch != "" {
		if !observed.BranchObservable {
			missing = append(missing, "local_branch_observable")
		}
		switch {
		case observed.RemoteReadable && observed.RemoteOID == "":
		case !req.KeepRemoteBranch:
			missing = append(missing, "remote_branch_absent")
		case observed.RemoteReadable:
			result.KeptRemoteBranch = &model.CleanupKeptRemoteBranch{Branch: inventory.Branch, RemoteOID: observed.RemoteOID, State: model.CleanupKeptRemoteBranchPresent}
		default:
			result.KeptRemoteBranch = &model.CleanupKeptRemoteBranch{Branch: inventory.Branch, State: model.CleanupKeptRemoteBranchUnreadable}
		}
	}
	result.Missing = missing
	result.OK = len(missing) == 0
	return result
}

// A missing prepared parent permits only a provider retarget to the observed
// default branch. Failed remote observation cannot erase known base drift.
func ClassifyCleanupMergedBase(prepared, observed, defaultBranch string, preparedPresent, observable bool) []string {
	if prepared == "" || observed == prepared {
		return nil
	}
	if !observable || strings.TrimSpace(defaultBranch) == "" {
		return []string{"base_branch_drifted", "merged_base_remote_unobserved"}
	}
	if !preparedPresent && observed == defaultBranch {
		return nil
	}
	return []string{"base_branch_drifted"}
}

func PreparedBaseBranch(record model.IssueOpsRecord) string {
	if record.BranchPrepare == nil {
		return ""
	}
	return strings.TrimSpace(record.BranchPrepare.BaseBranch)
}

func PreparedBaseRef(record model.IssueOpsRecord) string {
	base := strings.TrimPrefix(PreparedBaseBranch(record), "refs/heads/")
	return strings.TrimSpace(strings.TrimPrefix(base, "origin/"))
}

func ValidateCleanupSupersedeInput(record model.IssueOpsRecord, candidate string, configured bool) error {
	if strings.TrimSpace(candidate) == "" {
		return fmt.Errorf("no superseding artifact was provided")
	}
	if !configured {
		return fmt.Errorf("superseding artifact cannot be verified: provider observation is not configured")
	}
	if record.RemoteArtifact == nil || strings.TrimSpace(record.RemoteArtifact.URL) == "" {
		return fmt.Errorf("original artifact URL is unknown; cannot verify a supersede relation")
	}
	return nil
}
