package issueopscleanup

import (
	"context"
	"strings"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	lease "issueops/internal/domain/issueopslease"
	"issueops/internal/port"
)

type AbandonWorktreeObservation struct {
	Canonical, Clean, HeadObservable bool
	Branch, Head                     string
}
type AbandonEnvironment interface {
	Directory(string) (bool, error)
	SamePath(string, string) bool
	SameLinkedPath(string, string) bool
	Worktree(context.Context, string) AbandonWorktreeObservation
	BranchOID(context.Context, string, string) (string, error)
	BranchCheckoutPath(context.Context, string, string) (string, error)
}

type AbandonPreviewer struct {
	Environment AbandonEnvironment
	ReadChild   func(string) (model.IssueOpsRecord, error)
	Workspace   func(context.Context, model.IssueOpsRecord, string) (FinishWorkspaceObservation, []string)
	Orca        AbandonOrcaObserver
	Remote      AbandonRemoteObserver
}

func (s AbandonPreviewer) Plan(ctx context.Context, record model.IssueOpsRecord, req model.CleanupAbandonRequest, provider port.IssueProvider) (model.CleanupAbandonInventory, model.CleanupAbandonResult) {
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
		inventory.WorkspaceProcesses, inventory.OrcaTerminals = workspace.Receipts, workspace.Terminals
		inventory.OrcaAppPID, inventory.OrcaRuntimeReady = workspace.AppPID, workspace.RuntimeReady
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
	if record.Execution != nil && record.Execution.Pending != nil {
		if err := s.Orca.PendingSafe(ctx, record, inventory.WorktreePresent); err != nil {
			observed.PendingIntentError = domain.CleanupAbandonPendingRecovery(record.ID, err)
		}
	}
	if err := s.Orca.ResourcesAbsent(ctx, record, observed.LeaseHolderless, inventory.WorktreePresent && len(inventory.OrcaTerminals) > 0); err != nil {
		observed.OrcaResidueError = err.Error()
	}
	inventory, remote := s.Remote.Observe(ctx, record, req, provider, inventory)
	observed.RemoteMissing, observed.RemoteEffects = remote.RemoteMissing, remote.RemoteEffects
	observed.RemoteArtifactState, observed.IssueState = remote.RemoteArtifactState, remote.IssueState
	return domain.BuildCleanupAbandonPreview(record, req, inventory, observed)
}

func (s AbandonPreviewer) ResolvedChildren(record model.IssueOpsRecord) map[string]bool {
	children := []model.IssueOpsRecord{}
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
