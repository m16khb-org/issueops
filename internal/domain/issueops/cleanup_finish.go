package issueops

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

// CleanupFinishObservation supplies the facts cleanup finish observes outside
// the domain: the default branch, worktree and workspace state, and the
// readability and OID of the remote branch. A failed observation stays
// distinct from an absent resource.
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

// CleanupFinishTargets builds the local part of the cleanup finish inventory
// from record: the branch (the workspace branch wins), the remote artifact
// URL, and the worktree root (falling back to the recorded worktree path).
// Filesystem and git observations are filled in by the caller.
func CleanupFinishTargets(record model.Record) model.CleanupFinishInventory {
	inventory := model.CleanupFinishInventory{ID: record.ID, Repo: record.Repo, Branch: strings.TrimSpace(record.Branch)}
	if record.RemoteArtifact != nil {
		inventory.RemoteURL = record.RemoteArtifact.URL
	}
	if record.Execution != nil {
		inventory.WorktreeRoot = strings.TrimSpace(record.Execution.Workspace.Root)
		if branch := strings.TrimSpace(record.Execution.Workspace.Branch); branch != "" {
			inventory.Branch = branch
		}
	}
	if inventory.WorktreeRoot == "" {
		inventory.WorktreeRoot = strings.TrimSpace(record.WorktreePath)
	}
	return inventory
}

// BuildCleanupFinishPreview evaluates every cleanup finish gate against
// record, the request, the inventory, and the observed facts. The result
// lists the failed gates in Missing and is OK only when none failed. It also
// reports a retargeted base branch and a kept remote branch when the request
// allows them.
func BuildCleanupFinishPreview(record model.Record, req model.CleanupFinishRequest, inventory model.CleanupFinishInventory, observed CleanupFinishObservation) model.CleanupFinishResult {
	result := model.CleanupFinishResult{
		OK: true, ID: record.ID, Preview: !req.Apply, Missing: []string{},
		WorktreePath: inventory.WorktreeRoot, Branch: inventory.Branch,
		WorktreePresent: inventory.WorktreePresent, BranchPresent: inventory.BranchOID != "",
		WorkspaceProcesses: observed.Occupants, SupersededBy: inventory.SupersededBy, SupersedeError: observed.SupersedeError,
	}
	missing := cleanupFinishLifecycleMissing(result.Missing, record, req, inventory, observed)
	missing = cleanupFinishBaseMissing(missing, &result, record, req, inventory, observed)
	if hasUnclosedChildTask(record.IssueLinks) {
		missing = append(missing, "child_tasks_closed")
	}
	if observed.WorktreeIdentityConflict {
		missing = append(missing, "worktree_identity_conflict")
	}
	if observed.WorktreeUnobservable {
		missing = append(missing, "worktree_observable")
	}
	if inventory.WorktreePresent {
		missing = cleanupFinishWorktreeMissing(missing, req, observed)
	}
	if inventory.Branch != "" {
		missing = cleanupFinishBranchMissing(missing, &result, req, inventory, observed)
	}
	result.Missing = missing
	result.OK = len(missing) == 0
	return result
}

// cleanupFinishLifecycleMissing checks phase, lease, merge, completion, and issue-closed gates.
func cleanupFinishLifecycleMissing(missing []string, record model.Record, req model.CleanupFinishRequest, inventory model.CleanupFinishInventory, observed CleanupFinishObservation) []string {
	if record.Phase != model.PhaseDone {
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
	return missing
}

// cleanupFinishBaseMissing classifies the merged base branch and records a retargeted base when permitted.
func cleanupFinishBaseMissing(missing []string, result *model.CleanupFinishResult, record model.Record, req model.CleanupFinishRequest, inventory model.CleanupFinishInventory, observed CleanupFinishObservation) []string {
	prepared := PreparedBaseBranch(record)
	if prepared == "" {
		return missing
	}
	base := strings.TrimSpace(req.MergedBaseBranch)
	switch {
	case base == "":
		missing = append(missing, "merged_base_branch_unobserved")
	case inventory.SupersededBy != "":
	default:
		if slugs := classifyCleanupMergedBase(prepared, base, observed.DefaultBranch, observed.PreparedBasePresent, observed.BaseObserved); len(slugs) > 0 {
			missing = append(missing, slugs...)
		} else if base != prepared {
			result.RetargetedBase = &model.CleanupRetargetedBase{PreparedBase: prepared, ObservedBase: base, DefaultBranch: observed.DefaultBranch, PreparedBaseRemoteAbsent: true}
		}
	}
	return missing
}

// hasUnclosedChildTask reports whether any child link lacks close verification.
func hasUnclosedChildTask(links []model.IssueLink) bool {
	for _, link := range links {
		if link.Type == "child" && strings.TrimSpace(link.CloseVerifiedAt) == "" {
			return true
		}
	}
	return false
}

// cleanupFinishWorktreeMissing checks cwd placement, workspace leftovers, and cleanliness of a present worktree.
func cleanupFinishWorktreeMissing(missing []string, req model.CleanupFinishRequest, observed CleanupFinishObservation) []string {
	if strings.TrimSpace(req.CWD) == "" {
		missing = append(missing, "cwd_unresolved")
	} else if observed.CWDWithin {
		missing = append(missing, "cwd_outside_worktree")
	}
	missing = append(missing, observed.WorkspaceMissing...)
	if !observed.WorktreeClean {
		missing = append(missing, "worktree_clean")
	}
	return missing
}

// cleanupFinishBranchMissing checks local branch observability and remote branch absence or keep-state.
func cleanupFinishBranchMissing(missing []string, result *model.CleanupFinishResult, req model.CleanupFinishRequest, inventory model.CleanupFinishInventory, observed CleanupFinishObservation) []string {
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
	return missing
}

// A missing prepared parent permits only a provider retarget to the observed
// default branch. Failed remote observation cannot erase known base drift.
func classifyCleanupMergedBase(prepared, observed, defaultBranch string, preparedPresent, observable bool) []string {
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

// PreparedBaseBranch returns the trimmed base branch sealed by branch
// prepare, or an empty string when the record has no preparation.
func PreparedBaseBranch(record model.Record) string {
	if record.BranchPrepare == nil {
		return ""
	}
	return strings.TrimSpace(record.BranchPrepare.BaseBranch)
}

// PreparedBaseRef returns the prepared base branch stripped of a refs/heads/
// or origin/ prefix, so it can be compared with a remote branch name.
func PreparedBaseRef(record model.Record) string {
	base := strings.TrimPrefix(PreparedBaseBranch(record), "refs/heads/")
	return strings.TrimSpace(strings.TrimPrefix(base, "origin/"))
}

// ValidateCleanupSupersedeInput checks that a superseding artifact can be
// verified: a candidate was given, provider observation is configured, and
// the record knows the original artifact URL.
func ValidateCleanupSupersedeInput(record model.Record, candidate string, configured bool) error {
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
