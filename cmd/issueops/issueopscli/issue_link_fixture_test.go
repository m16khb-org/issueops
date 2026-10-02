package issueopscli

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
func LinkIssueOpsChildWithActorForTest(root, id, childURL, title string, actor model.IssueOpsActor) (model.IssueOpsRecord, error) {
	return issueLinkerForTest(root).Child(context.Background(), id, childURL, title, &actor)
}
func LinkIssueOpsIssueForTest(root, id, issueURL string) (model.IssueOpsRecord, error) {
	return issueLinkerForTest(root).Issue(context.Background(), id, issueURL, nil)
}
func LinkIssueOpsIssueWithActorForTest(root, id, issueURL string, actor model.IssueOpsActor) (model.IssueOpsRecord, error) {
	return issueLinkerForTest(root).Issue(context.Background(), id, issueURL, &actor)
}
func LinkIssueOpsRelatedWithActorForTest(root, id, linkType, relatedURL, title string, actor model.IssueOpsActor) (model.IssueOpsRecord, error) {
	return issueLinkerForTest(root).Related(context.Background(), id, linkType, relatedURL, title, &actor)
}
