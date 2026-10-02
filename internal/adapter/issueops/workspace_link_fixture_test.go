package issueops

import (
	"context"
	"time"

	authorizationoutbound "issueops/internal/adapter/outbound/issueopsauthorization"
	branchapp "issueops/internal/application/issueopsbranch"
	cycleapp "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
)

func workspaceLinkerForTest(root string) branchapp.WorkspaceLinker {
	return branchapp.WorkspaceLinker{Records: CycleRecordStore{StateRoot: root}, Authority: cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same, liveTestVerifier()), Files: LinkEnvironment{}, Now: time.Now}
}
func LinkIssueOpsPlan(root, id, planPath string) (model.IssueOpsRecord, error) {
	return workspaceLinkerForTest(root).Plan(context.Background(), id, planPath, nil)
}
func LinkIssueOpsPlanWithActor(root, id, planPath string, actor model.IssueOpsActor) (model.IssueOpsRecord, error) {
	return workspaceLinkerForTest(root).Plan(context.Background(), id, planPath, &actor)
}
func LinkIssueOpsWorktree(root, id, worktreePath string) (model.IssueOpsRecord, error) {
	return workspaceLinkerForTest(root).Worktree(context.Background(), id, worktreePath, nil)
}
