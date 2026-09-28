package issueopsapp

import (
	"time"

	core "issueops/internal/adapter/issueops"
	authorizationoutbound "issueops/internal/adapter/outbound/issueopsauthorization"
	cycleapp "issueops/internal/application/issueopscycle"
	delegationapp "issueops/internal/application/issueopsdelegation"
)

func newChildStarter(root string) delegationapp.ChildStarter {
	return delegationapp.ChildStarter{
		Records:   core.ChildCycleStore{CycleRecordStore: core.CycleRecordStore{StateRoot: root}},
		Identity:  core.CycleStartIdentity{},
		Authority: cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same),
		Links:     newIssueLinker(root),
		Now:       time.Now,
	}
}
