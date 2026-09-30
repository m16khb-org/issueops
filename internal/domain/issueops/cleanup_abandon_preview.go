package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

// CleanupAbandonObservation supplies facts gathered outside the domain. Failed
// observations remain distinct from absent resources.
type CleanupAbandonObservation struct {
	LeaseHolderless                                                 bool
	ResolvedChildren                                                map[string]bool
	WorktreeIdentityConflict, WorktreeUnobservable                  bool
	WorktreeHeadObservable                                          bool
	WorkspaceMissing                                                []string
	Occupants                                                       []model.CleanupWorkspaceProcess
	BranchObservable, RegistryObservable, BranchCheckedOutElsewhere bool
	FailureEvidence                                                 CleanupAbandonFailureEvidence
	PendingIntentError, OrcaResidueError                            string
	RemoteMissing, RemoteEffects                                    []string
	RemoteArtifactState, IssueState                                 string
}

func BuildCleanupAbandonPreview(record model.IssueOpsRecord, req model.CleanupAbandonRequest, inventory model.CleanupAbandonInventory, observed CleanupAbandonObservation) (model.CleanupAbandonInventory, model.CleanupAbandonResult) {
	result := model.CleanupAbandonResult{
		OK: true, ID: record.ID, Preview: !req.Apply, Reason: strings.TrimSpace(req.Reason),
		RemoteBranchDeletion: "not_planned", Missing: []string{},
		WorktreePath: inventory.WorktreeRoot, Branch: inventory.Branch,
		WorktreePresent: inventory.WorktreePresent, WorktreeCanonical: inventory.WorktreeCanonical,
		WorktreeClean: inventory.WorktreeClean, WorktreeHead: inventory.WorktreeHead,
		BranchPresent: inventory.BranchOID != "", BranchOID: inventory.BranchOID,
		BranchCheckoutPath: inventory.BranchCheckoutPath, PendingOperationID: inventory.PendingOperationID,
		WorkspaceProcesses: observed.Occupants, OrcaTerminals: inventory.OrcaTerminals,
		RemoteBranchOID: inventory.RemoteBranchOID, RemoteArtifactState: observed.RemoteArtifactState, IssueState: observed.IssueState,
		RemoteEffects: observed.RemoteEffects,
	}
	missing := result.Missing
	if err := ValidateCleanupAbandonReason(req.Reason); err != nil {
		missing = append(missing, "reason_required")
		result.ReasonError = err.Error()
	}
	// A done phase without merged evidence can still be abandoned. Writer and
	// artifact evidence, rather than phase names, determine eligibility.
	if record.Execution != nil && !observed.LeaseHolderless {
		missing = append(missing, "lease_terminal")
	}
	if record.RemoteArtifact != nil && !req.ArtifactUnmerged {
		missing = append(missing, "remote_artifact_unmerged")
	}
	if unresolved := CleanupAbandonUnresolvedChildren(record, observed.ResolvedChildren); len(unresolved) > 0 {
		missing = append(missing, "no_children")
		result.UnresolvedChildren = unresolved
	}
	if observed.WorktreeIdentityConflict {
		missing = append(missing, "worktree_identity_conflict")
	}
	if observed.WorktreeUnobservable {
		missing = append(missing, "worktree_observable")
	}
	if inventory.WorktreePresent {
		if !inventory.WorktreeCanonical {
			missing = append(missing, "worktree_canonical")
		}
		if inventory.WorktreeBranch != inventory.Branch {
			missing = append(missing, "worktree_branch_match")
		}
		if !observed.WorktreeHeadObservable {
			missing = append(missing, "worktree_head")
		}
		if !inventory.WorktreeClean {
			missing = append(missing, "worktree_clean")
		}
		missing = append(missing, observed.WorkspaceMissing...)
	}
	if inventory.Branch != "" && !observed.BranchObservable {
		missing = append(missing, "local_branch_observable")
	}
	if result.BranchPresent {
		if !observed.RegistryObservable {
			missing = append(missing, "worktree_registry_observable")
		} else if observed.BranchCheckedOutElsewhere {
			missing = append(missing, "branch_checked_out_elsewhere")
		}
	}
	receiptMatches := CleanupAbandonFailureInventoryMatches(record, inventory, observed.FailureEvidence)
	if record.CleanupAbandonFailure != nil && !receiptMatches {
		missing = append(missing, "cleanup_failure_inventory")
	}
	if receiptMatches && CleanupAbandonPartialBranchRetry(record, inventory, observed.FailureEvidence) {
		inventory.WorktreeHead = record.CleanupAbandonFailure.WorktreeHead
		result.WorktreeHead = inventory.WorktreeHead
	}
	if (inventory.WorktreePresent || result.BranchPresent) && record.Execution == nil && strings.TrimSpace(record.WorktreePath) == "" {
		missing = append(missing, "local_residue_execution")
	}
	// Asymmetric residue is allowed; when both remain they must identify the
	// same commit. The fingerprint binds the two observed resource axes.
	if inventory.WorktreePresent && result.BranchPresent && inventory.WorktreeHead != inventory.BranchOID {
		missing = append(missing, "local_branch_head")
	}
	if observed.PendingIntentError != "" {
		missing = append(missing, "pending_intent_safe")
		result.PendingIntentError = observed.PendingIntentError
	}
	if observed.OrcaResidueError != "" {
		missing = append(missing, "orca_resources_absent")
		result.OrcaResidueError = observed.OrcaResidueError
	}
	missing = append(missing, observed.RemoteMissing...)
	result.Missing = missing
	result.OK = len(missing) == 0
	return inventory, result
}
