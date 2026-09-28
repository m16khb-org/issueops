package remotecmd

import (
	"context"
	core "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/issueops/branchinstructions"
	authorizationoutbound "issueops/internal/adapter/outbound/issueopsauthorization"
	branchapp "issueops/internal/application/issueopsbranch"
	cycleapp "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
	"time"
)

func branchPreparerForTest(root string) branchapp.Preparer {
	environment := core.BranchPreparationEnvironment{}
	return branchapp.Preparer{Records: core.CycleRecordStore{StateRoot: root}, Authority: cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same), CleanParentPath: environment.CleanParentPath, ResolveBaseCommit: environment.ResolveBaseCommit, UmbrellaForChildIssue: environment.UmbrellaForChildIssue, ObserveCodeProjectKey: environment.ObserveCodeProjectKey, Steps: branchinstructions.Steps, Now: time.Now}
}
func prepareBranchForTest(root, id string, req model.IssueOpsBranchPrepareRequest) (model.IssueOpsRecord, error) {
	return branchPreparerForTest(root).Prepare(context.Background(), id, req, nil)
}
