package issueopscli

import (
	"context"
	preflightadapter "issueops/internal/adapter/preflight"
	"time"

	core "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/issueops/branchinstructions"
	"issueops/internal/adapter/issueops/pathutil"
	authorizationoutbound "issueops/internal/adapter/outbound/issueopsauthorization"
	branchapp "issueops/internal/application/issueopsbranch"
	cycleapp "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
)

func branchPreparerForTest(root string) branchapp.Preparer {
	environment := core.BranchPreparationEnvironment{RunGit: preflightadapter.GitCmd}
	return branchapp.Preparer{Records: core.CycleRecordStore{StateRoot: root}, Authority: cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same), CleanParentPath: environment.CleanParentPath, ResolveBaseCommit: environment.ResolveBaseCommit, UmbrellaForChildIssue: (branchapp.ActiveCycleReader{Scan: func() ([]model.IssueOpsRecord, error) { return core.ScanReadableIssueOps(root) }, CleanPath: pathutil.CleanAbsPath}).UmbrellaForChildIssue, ObserveCodeProjectKey: environment.ObserveCodeProjectKey, Steps: branchinstructions.Steps, Now: time.Now}
}
func prepareBranchWithActorForTest(root, id string, req model.IssueOpsBranchPrepareRequest, actor model.IssueOpsActor) (model.IssueOpsRecord, error) {
	return branchPreparerForTest(root).Prepare(context.Background(), id, req, &actor)
}
func prepareBranchForTest(root, id string, req model.IssueOpsBranchPrepareRequest) (model.IssueOpsRecord, error) {
	return branchPreparerForTest(root).Prepare(context.Background(), id, req, nil)
}
