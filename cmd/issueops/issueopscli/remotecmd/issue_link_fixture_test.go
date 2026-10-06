package remotecmd

import (
	"context"
	"time"

	core "issueops/internal/adapter/issueops"
	authorizationoutbound "issueops/internal/adapter/outbound/issueopsauthorization"
	branchapp "issueops/internal/application/issueopsbranch"
	cycleapp "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
)

func issueLinkerForTest(root string) branchapp.Linker {
	return branchapp.Linker{Records: core.CycleRecordStore{StateRoot: root}, Authority: cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same, core.NativeActorVerifier()), Now: time.Now}
}
func LinkIssueOpsChildForTest(root, id, childURL, title string) (model.IssueOpsRecord, error) {
	return issueLinkerForTest(root).Child(context.Background(), id, childURL, title, nil)
}
func LinkIssueOpsIssueForTest(root, id, issueURL string) (model.IssueOpsRecord, error) {
	return issueLinkerForTest(root).Issue(context.Background(), id, issueURL, nil)
}
