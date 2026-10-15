package issueopsowner

import (
	"encoding/json"
	"fmt"

	model "issueops/internal/contract/issueops"
	prep "issueops/internal/contract/issueopspreparation"
	domain "issueops/internal/domain/issueops"
	remote "issueops/internal/domain/issueopsremote"
)

// ContextFiles는 workspace 해석과 봉인 아티팩트 materialize가 쓰는 파일 관측이다.
type ContextFiles interface {
	SamePath(string, string) bool
	Materialize(model.IssueOpsRecord) (map[string]string, error)
}

type Service struct {
	Files ContextFiles
}

func (s Service) ResolveWorkspace(snapshot prep.Snapshot, confirm bool) (prep.WorkspaceRequest, error) {
	record, err := executionPreparationCoreRecord(snapshot)
	if err != nil {
		return prep.WorkspaceRequest{}, err
	}
	layout, err := domain.DeriveWorkspaceLayout(record)
	if err != nil {
		return prep.WorkspaceRequest{}, err
	}
	if layout.ExpectedParent != "" {
		if err := domain.ValidateWorkspaceParent(layout, s.Files.SamePath(layout.ParentWorktree, layout.ExpectedParent)); err != nil {
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
	record.Execution = &model.Execution{Workspace: model.Workspace{SourceRoot: receipt.SourceRoot, Root: receipt.Root, Branch: receipt.Branch, BaseHead: receipt.BaseHead, ParentWorktree: receipt.ParentWorktree, ArtifactDir: OwnerArtifactDir(record)}}
	_, err = s.Files.Materialize(record)
	return err
}

func OwnerArtifactDir(record model.IssueOpsRecord) string {
	if dir := remote.IssueArtifactDir(record.IssueURL); dir != "" {
		return dir
	}
	if record.BranchPrepare != nil {
		return remote.IssueArtifactDir(record.BranchPrepare.IssueURL)
	}
	return ""
}

func executionPreparationCoreRecord(snapshot prep.Snapshot) (model.IssueOpsRecord, error) {
	if len(snapshot.RecordRaw) == 0 {
		return model.IssueOpsRecord{}, fmt.Errorf("preparation raw record snapshot is required")
	}
	var record model.IssueOpsRecord
	if err := json.Unmarshal(snapshot.RecordRaw, &record); err != nil {
		return model.IssueOpsRecord{}, err
	}
	return record, nil
}
