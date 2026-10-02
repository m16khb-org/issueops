package issueopsapp

import (
	core "issueops/internal/adapter/issueops"
	authorizationoutbound "issueops/internal/adapter/outbound/issueopsauthorization"
	branchapp "issueops/internal/application/issueopsbranch"
	cycleapp "issueops/internal/application/issueopscycle"
	"time"
)

func newIssueLinker(root string) branchapp.Linker {
	return branchapp.Linker{Records: core.CycleRecordStore{StateRoot: root}, Authority: cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same, issueOpsActorVerifier()), Now: time.Now}
}

func newWorkspaceLinker(root string) branchapp.WorkspaceLinker {
	return branchapp.WorkspaceLinker{Records: core.CycleRecordStore{StateRoot: root}, Authority: cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same, issueOpsActorVerifier()), Files: core.LinkEnvironment{}, Now: time.Now}
}
