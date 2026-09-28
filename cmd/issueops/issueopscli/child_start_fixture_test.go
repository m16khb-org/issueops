package issueopscli

import (
	"context"
	"time"

	core "issueops/internal/adapter/issueops"
	authorizationoutbound "issueops/internal/adapter/outbound/issueopsauthorization"
	cycleapp "issueops/internal/application/issueopscycle"
	delegationapp "issueops/internal/application/issueopsdelegation"
	model "issueops/internal/contract/issueops"
)

func startChildWithActorForTest(root string, req model.IssueOpsChildStartRequest, actor model.IssueOpsActor) (model.IssueOpsChildStartResult, error) {
	starter := delegationapp.ChildStarter{
		Records:   core.ChildCycleStore{CycleRecordStore: core.CycleRecordStore{StateRoot: root}},
		Identity:  core.CycleStartIdentity{},
		Authority: cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same),
		Links:     issueLinkerForTest(root),
		Now:       time.Now,
	}
	return starter.Start(context.Background(), req, &actor)
}
