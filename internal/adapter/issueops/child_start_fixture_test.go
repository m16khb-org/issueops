package issueops

import (
	"context"
	"time"

	branchapp "issueops/internal/application/issueopsbranch"
	cycleapp "issueops/internal/application/issueopscycle"
	delegationapp "issueops/internal/application/issueopsdelegation"
	model "issueops/internal/contract/issueops"
)

func childStarterForTest(root string) delegationapp.ChildStarter {
	records := CycleRecordStore{StateRoot: root}
	authority := cycleapp.NewMutationAuthority(samePath)
	return delegationapp.ChildStarter{Records: ChildCycleStore{CycleRecordStore: records}, Identity: CycleStartIdentity{}, Authority: authority, Links: branchapp.Linker{Records: records, Authority: authority, Now: time.Now}, Now: time.Now}
}
func StartIssueOpsChildWithActor(root string, req model.IssueOpsChildStartRequest, actor model.IssueOpsActor) (model.IssueOpsChildStartResult, error) {
	return childStarterForTest(root).Start(context.Background(), req, &actor)
}
