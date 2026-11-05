package issueopscleanup

import (
	"context"
	"strings"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	lease "issueops/internal/domain/issueopslease"
	"issueops/internal/port"
)

// AbandonWorktreeObservation is what an abandon environment reports about a
// local worktree: whether it is the canonical linked worktree, clean, and
// whether its HEAD could be read, plus the branch and HEAD it is on.
type AbandonWorktreeObservation struct {
	Canonical, Clean, HeadObservable bool
	Branch, Head                     string
}
type abandonEnvironment interface {
	Directory(string) (bool, error)
	SamePath(string, string) bool
	SameLinkedPath(string, string) bool
	Worktree(context.Context, string) AbandonWorktreeObservation
	BranchOID(context.Context, string, string) (string, error)
	BranchCheckoutPath(context.Context, string, string) (string, error)
}

// AbandonPreviewer builds the read-only abandon inventory and preview result
// from local git, workspace and remote observations without mutating anything.
type AbandonPreviewer struct {
	Environment abandonEnvironment
	ReadChild   func(string) (model.Record, error)
	Workspace   func(context.Context, model.Record, string) (FinishWorkspaceObservation, []string)
	Remote      AbandonRemoteObserver
}

// Plan observes the record's worktree, branch, workspace occupants, children
// and remote artifacts, then returns the inventory and the domain-built
// preview result, whose Missing list names the unmet gates.
func (s AbandonPreviewer) Plan(ctx context.Context, record model.Record, req model.CleanupAbandonRequest, provider port.IssueProvider) (model.CleanupAbandonInventory, model.CleanupAbandonResult) {
	inventory := domain.CleanupAbandonTargets(record)
	inventory.RecordSHA = CleanupAbandonRecordSHA(record)
	observed := domain.CleanupAbandonObservation{ResolvedChildren: s.ResolvedChildren(record)}
	if record.Execution != nil {
		observed.LeaseHolderless = !lease.LeaseHoldsWriter(string(record.Execution.Lease.Status))
	}
	if linked := strings.TrimSpace(record.WorktreePath); linked != "" {
		observed.WorktreeIdentityConflict = !s.Environment.SameLinkedPath(inventory.WorktreeRoot, linked)
	}
	if inventory.WorktreeRoot != "" {
		var err error
		inventory.WorktreePresent, err = s.Environment.Directory(inventory.WorktreeRoot)
		observed.WorktreeUnobservable = err != nil
	}
	if inventory.WorktreePresent {
		local := s.Environment.Worktree(ctx, inventory.WorktreeRoot)
		inventory.WorktreeCanonical, inventory.WorktreeClean = local.Canonical, local.Clean
		inventory.WorktreeBranch, inventory.WorktreeHead = local.Branch, local.Head
		observed.WorktreeHeadObservable = local.HeadObservable
		workspace, missing := s.Workspace(ctx, record, inventory.WorktreeRoot)
		observed.WorkspaceMissing, observed.Occupants = missing, workspace.Occupants
		inventory.WorkspaceProcesses = workspace.Receipts
	}
	if inventory.Branch != "" {
		var err error
		inventory.BranchOID, err = s.Environment.BranchOID(ctx, record.Repo, inventory.Branch)
		observed.BranchObservable = err == nil
	}
	if inventory.BranchOID != "" {
		var err error
		inventory.BranchCheckoutPath, err = s.Environment.BranchCheckoutPath(ctx, record.Repo, inventory.Branch)
		observed.RegistryObservable = err == nil
		observed.BranchCheckedOutElsewhere = inventory.BranchCheckoutPath != "" && !s.Environment.SamePath(inventory.BranchCheckoutPath, inventory.WorktreeRoot)
	}
	if failure := record.CleanupAbandonFailure; failure != nil {
		observed.FailureEvidence.SameWorktree = s.Environment.SamePath(failure.WorktreePath, inventory.WorktreeRoot)
		observed.FailureEvidence.SealMatches = failure.InventorySHA256 == CleanupAbandonFailureSeal(record, failure)
	}
	observed.PendingIntentError = domain.CleanupAbandonPendingError(record)
	inventory, remote := s.Remote.Observe(ctx, record, req, provider, inventory)
	observed.RemoteMissing, observed.RemoteEffects = remote.RemoteMissing, remote.RemoteEffects
	observed.RemoteArtifactState, observed.IssueState = remote.RemoteArtifactState, remote.IssueState
	return domain.BuildCleanupAbandonPreview(record, req, inventory, observed)
}

// ResolvedChildren reports, per child cycle ID, whether the child record could
// be read and is resolved. Child cycles that cannot be read are omitted.
func (s AbandonPreviewer) ResolvedChildren(record model.Record) map[string]bool {
	children := []model.Record{}
	for _, child := range record.ChildCycles {
		id := strings.TrimSpace(child.CycleID)
		if id == "" {
			continue
		}
		observed, err := s.ReadChild(id)
		if err == nil {
			children = append(children, observed)
		}
	}
	return domain.CleanupAbandonResolvedChildren(children)
}
