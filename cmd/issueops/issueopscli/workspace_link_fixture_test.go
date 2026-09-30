package issueopscli

import (
	"context"
	"time"

	core "issueops/internal/adapter/issueops"
	authorizationoutbound "issueops/internal/adapter/outbound/issueopsauthorization"
	branchapp "issueops/internal/application/issueopsbranch"
	cycleapp "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
)

func workspaceLinkerForTest(root string) branchapp.WorkspaceLinker {
	return branchapp.WorkspaceLinker{Records: core.CycleRecordStore{StateRoot: root}, Authority: cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same), Files: core.LinkEnvironment{}, Now: time.Now}
}
func LinkIssueOpsPlanForTest(root, id, planPath string) (model.IssueOpsRecord, error) {
	return workspaceLinkerForTest(root).Plan(context.Background(), id, planPath, nil)
}
func LinkIssueOpsPlanWithActorForTest(root, id, planPath string, actor model.IssueOpsActor) (model.IssueOpsRecord, error) {
	return workspaceLinkerForTest(root).Plan(context.Background(), id, planPath, &actor)
}
func LinkIssueOpsWorktreeForTest(root, id, worktreePath string) (model.IssueOpsRecord, error) {
	return workspaceLinkerForTest(root).Worktree(context.Background(), id, worktreePath, nil)
}
func LinkIssueOpsWorktreeWithActorForTest(root, id, worktreePath string, actor model.IssueOpsActor) (model.IssueOpsRecord, error) {
	return workspaceLinkerForTest(root).Worktree(context.Background(), id, worktreePath, &actor)
}
