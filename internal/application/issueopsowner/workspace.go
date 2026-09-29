package issueopsowner

import (
	model "issueops/internal/contract/issueops"
	prep "issueops/internal/contract/issueopspreparation"
	domain "issueops/internal/domain/issueops"
)

func (s Service) ResolveWorkspace(snapshot prep.Snapshot, confirm bool) (prep.WorkspaceRequest, error) {
	record, err := executionPreparationCoreRecord(snapshot)
	if err != nil {
		return prep.WorkspaceRequest{}, err
	}
	layout, err := domain.OwnerWorkspaceLayout(record)
	if err != nil {
		return prep.WorkspaceRequest{}, err
	}
	if layout.ExpectedParent != "" {
		if err := domain.ValidateOwnerWorkspaceParent(layout, s.Files.SamePath(layout.ParentWorktree, layout.ExpectedParent)); err != nil {
			return prep.WorkspaceRequest{}, err
		}
	}
	return prep.WorkspaceRequest{LifecycleID: record.ID, SourceRoot: record.Repo, Root: layout.Root, Branch: layout.Branch, BaseBranch: layout.BaseBranch, BaseHead: layout.BaseHead, ParentWorktree: layout.ParentWorktree, Confirm: confirm}, nil
}
func (s Service) MaterializeDirect(snapshot prep.Snapshot, receipt prep.WorkspaceReceipt) error {
	record, err := executionPreparationCoreRecord(snapshot)
	if err != nil {
		return err
	}
	record.WorktreePath = receipt.Root
	record.Execution = &model.Execution{Mode: model.ExecutionModeDirect, Workspace: model.Workspace{SourceRoot: receipt.SourceRoot, Root: receipt.Root, Branch: receipt.Branch, BaseHead: receipt.BaseHead, ParentWorktree: receipt.ParentWorktree, Driver: receipt.Driver, ArtifactDir: OwnerArtifactDir(record)}}
	_, err = s.Files.Materialize(record)
	return err
}
