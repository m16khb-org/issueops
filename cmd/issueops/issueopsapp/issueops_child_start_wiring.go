package issueopsapp

import (
	branchpreflight "issueops/internal/adapter/preflight"
	"time"

	core "issueops/internal/adapter/issueops"
	authorizationoutbound "issueops/internal/adapter/outbound/issueopsauthorization"
	cycleapp "issueops/internal/application/issueopscycle"
	delegationapp "issueops/internal/application/issueopsdelegation"
)

func newChildStarter(root string) delegationapp.ChildStarter {
	return delegationapp.ChildStarter{
		Records:   core.ChildCycleStore{CycleRecordStore: core.CycleRecordStore{StateRoot: root}},
		Identity:  core.CycleStartIdentity{RunGit: branchpreflight.GitCmd},
		Authority: cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same, issueOpsActorVerifier()),
		Links:     newIssueLinker(root),
		Now:       time.Now,
	}
}

func newChildStatusService(root string) delegationapp.StatusService {
	return delegationapp.StatusService{
		Records:   core.ChildCycleStore{CycleRecordStore: core.CycleRecordStore{StateRoot: root}},
		Authority: cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same, issueOpsActorVerifier()),
		Now:       time.Now,
	}
}

func newChildValidator(root string) delegationapp.Validator {
	return delegationapp.Validator{Records: core.CycleRecordStore{StateRoot: root}, Authority: cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same, issueOpsActorVerifier()), Now: time.Now}
}
