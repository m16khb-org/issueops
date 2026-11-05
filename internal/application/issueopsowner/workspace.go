package issueopsowner

import (
	"encoding/json"
	"fmt"

	model "issueops/internal/contract/issueops"
	prep "issueops/internal/contract/issueopspreparation"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/domain/issueopsremote"
)

// contextFiles는 workspace 해석과 봉인 아티팩트 materialize가 쓰는 파일 관측이다.
type contextFiles interface {
	SamePath(string, string) bool
	Materialize(model.Record) (map[string]string, error)
}

// Service 타입은 준비 스냅샷에서 workspace 요청을 만들고 봉인 아티팩트를
// materialize한다. 파일 관측은 Files로 주입한다.
type Service struct {
	Files contextFiles
}

// ResolveWorkspace 메서드는 snapshot의 원본 레코드에서 workspace 배치를 도출하고,
// 부모 worktree가 기대 경로와 같은지 검증한 뒤 준비 요청을 돌려준다. snapshot에
// 원본 레코드가 없으면 오류다.
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

// MaterializeDirect 메서드는 receipt의 worktree를 가리키는 execution을 레코드에
// 채워 봉인 아티팩트를 Files.Materialize로 내려쓴다. 저장소 레코드는 바꾸지 않는다.
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

// OwnerArtifactDir 함수는 레코드의 이슈 URL(없으면 branch prepare에 기록된 이슈 URL)에서
// 봉인 아티팩트 디렉터리를 도출한다. 이슈 번호가 없으면 빈 문자열이다.
func OwnerArtifactDir(record model.Record) string {
	if dir := issueopsremote.IssueArtifactDir(record.IssueURL); dir != "" {
		return dir
	}
	if record.BranchPrepare != nil {
		return issueopsremote.IssueArtifactDir(record.BranchPrepare.IssueURL)
	}
	return ""
}

func executionPreparationCoreRecord(snapshot prep.Snapshot) (model.Record, error) {
	if len(snapshot.RecordRaw) == 0 {
		return model.Record{}, fmt.Errorf("preparation raw record snapshot is required")
	}
	var record model.Record
	if err := json.Unmarshal(snapshot.RecordRaw, &record); err != nil {
		return model.Record{}, err
	}
	return record, nil
}
