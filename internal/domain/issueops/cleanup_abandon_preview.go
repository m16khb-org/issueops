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
	b := &cleanupAbandonPreviewBuilder{record: &record, req: &req, inventory: inventory, observed: &observed}
	b.result = model.CleanupAbandonResult{
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
	b.missing = b.result.Missing
	b.checkReason()
	b.checkLeaseArtifactAndChildren()
	b.checkWorktreeObservation()
	b.checkBranchObservation()
	b.checkFailureReceipt()
	b.checkLocalResidue()
	b.checkPendingIntentOrcaAndRemote()
	b.result.Missing = b.missing
	b.result.OK = len(b.missing) == 0
	return b.inventory, b.result
}

// cleanupAbandonPreviewBuilder accumulates the missing-gate list in the order
// the preview checks run; the order is part of the observable result.
type cleanupAbandonPreviewBuilder struct {
	record    *model.IssueOpsRecord
	req       *model.CleanupAbandonRequest
	inventory model.CleanupAbandonInventory
	observed  *CleanupAbandonObservation
	result    model.CleanupAbandonResult
	missing   []string
}

func (b *cleanupAbandonPreviewBuilder) add(gate string) {
	b.missing = append(b.missing, gate)
}

func (b *cleanupAbandonPreviewBuilder) checkReason() {
	if err := ValidateCleanupAbandonReason(b.req.Reason); err != nil {
		b.add("reason_required")
		b.result.ReasonError = err.Error()
	}
}

func (b *cleanupAbandonPreviewBuilder) checkLeaseArtifactAndChildren() {
	// A done phase without merged evidence can still be abandoned. Writer and
	// artifact evidence, rather than phase names, determine eligibility.
	if b.record.Execution != nil && !b.observed.LeaseHolderless {
		b.add("lease_terminal")
	}
	if b.record.RemoteArtifact != nil && !b.req.ArtifactUnmerged {
		b.add("remote_artifact_unmerged")
	}
	if unresolved := CleanupAbandonUnresolvedChildren(*b.record, b.observed.ResolvedChildren); len(unresolved) > 0 {
		b.add("no_children")
		b.result.UnresolvedChildren = unresolved
	}
}

func (b *cleanupAbandonPreviewBuilder) checkWorktreeObservation() {
	if b.observed.WorktreeIdentityConflict {
		b.add("worktree_identity_conflict")
	}
	if b.observed.WorktreeUnobservable {
		b.add("worktree_observable")
	}
	if !b.inventory.WorktreePresent {
		return
	}
	for _, check := range []struct {
		failed bool
		gate   string
	}{
		{!b.inventory.WorktreeCanonical, "worktree_canonical"},
		{b.inventory.WorktreeBranch != b.inventory.Branch, "worktree_branch_match"},
		{!b.observed.WorktreeHeadObservable, "worktree_head"},
		{!b.inventory.WorktreeClean, "worktree_clean"},
	} {
		if check.failed {
			b.add(check.gate)
		}
	}
	b.missing = append(b.missing, b.observed.WorkspaceMissing...)
}

func (b *cleanupAbandonPreviewBuilder) checkBranchObservation() {
	if b.inventory.Branch != "" && !b.observed.BranchObservable {
		b.add("local_branch_observable")
	}
	if !b.result.BranchPresent {
		return
	}
	if !b.observed.RegistryObservable {
		b.add("worktree_registry_observable")
	} else if b.observed.BranchCheckedOutElsewhere {
		b.add("branch_checked_out_elsewhere")
	}
}

func (b *cleanupAbandonPreviewBuilder) checkFailureReceipt() {
	receiptMatches := CleanupAbandonFailureInventoryMatches(*b.record, b.inventory, b.observed.FailureEvidence)
	if b.record.CleanupAbandonFailure != nil && !receiptMatches {
		b.add("cleanup_failure_inventory")
	}
	if receiptMatches && CleanupAbandonPartialBranchRetry(*b.record, b.inventory, b.observed.FailureEvidence) {
		b.inventory.WorktreeHead = b.record.CleanupAbandonFailure.WorktreeHead
		b.result.WorktreeHead = b.inventory.WorktreeHead
	}
}

func (b *cleanupAbandonPreviewBuilder) checkLocalResidue() {
	if (b.inventory.WorktreePresent || b.result.BranchPresent) && b.record.Execution == nil && strings.TrimSpace(b.record.WorktreePath) == "" {
		b.add("local_residue_execution")
	}
	// Asymmetric residue is allowed; when both remain they must identify the
	// same commit. The fingerprint binds the two observed resource axes.
	if b.inventory.WorktreePresent && b.result.BranchPresent && b.inventory.WorktreeHead != b.inventory.BranchOID {
		b.add("local_branch_head")
	}
}

func (b *cleanupAbandonPreviewBuilder) checkPendingIntentOrcaAndRemote() {
	if b.observed.PendingIntentError != "" {
		b.add("pending_intent_safe")
		b.result.PendingIntentError = b.observed.PendingIntentError
	}
	if b.observed.OrcaResidueError != "" {
		b.add("orca_resources_absent")
		b.result.OrcaResidueError = b.observed.OrcaResidueError
	}
	b.missing = append(b.missing, b.observed.RemoteMissing...)
}
