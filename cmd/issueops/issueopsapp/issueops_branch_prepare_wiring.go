package issueopsapp

import (
	preflightadapter "issueops/internal/adapter/preflight"
	"time"

	core "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/issueops/branchinstructions"
	authorizationoutbound "issueops/internal/adapter/outbound/issueopsauthorization"
	branchapp "issueops/internal/application/issueopsbranch"
	cycleapp "issueops/internal/application/issueopscycle"
)

func newBranchPreparer(root string) branchapp.Preparer {
	environment := core.BranchPreparationEnvironment{RunGit: preflightadapter.GitCmd}
	return branchapp.Preparer{
		Records:               core.CycleRecordStore{StateRoot: root},
		Authority:             cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same, issueOpsActorVerifier()),
		CleanParentPath:       environment.CleanParentPath,
		ResolveBaseCommit:     environment.ResolveBaseCommit,
		UmbrellaForChildIssue: newActiveCycleReader(root).UmbrellaForChildIssue,
		ObserveCodeProjectKey: environment.ObserveCodeProjectKey,
		Steps:                 branchinstructions.Steps, Now: time.Now,
	}
}
