package issueopscli

import (
	"context"
	branchpreflight "issueops/internal/adapter/preflight"
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
		Identity:  core.CycleStartIdentity{RunGit: branchpreflight.GitCmd},
		Authority: cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same, core.NativeActorVerifier()),
		Links:     issueLinkerForTest(root),
		Now:       time.Now,
	}
	return starter.Start(context.Background(), req, &actor)
}

func childStatusWithActorForTest(root, id string, repair bool, actor model.IssueOpsActor) (model.IssueOpsChildStatusResult, error) {
	service := delegationapp.StatusService{
		Records:   core.ChildCycleStore{CycleRecordStore: core.CycleRecordStore{StateRoot: root}},
		Authority: cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same, core.NativeActorVerifier()),
		Now:       time.Now,
	}
	return service.Status(context.Background(), id, repair, &actor)
}

func childValidatorForTest(root string) delegationapp.Validator {
	return delegationapp.Validator{Records: core.CycleRecordStore{StateRoot: root}, Authority: cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same, core.NativeActorVerifier()), Now: time.Now}
}
func acceptChildWithActorForTest(root, parentID, childID string, evidence []string, actor model.IssueOpsActor) (model.IssueOpsChildValidationResult, error) {
	return childValidatorForTest(root).Accept(context.Background(), parentID, childID, evidence, &actor)
}
func rejectChildWithActorForTest(root, parentID, childID, reason string, evidence []string, actor model.IssueOpsActor) (model.IssueOpsChildValidationResult, error) {
	return childValidatorForTest(root).Reject(context.Background(), parentID, childID, reason, evidence, &actor)
}
func dropChildWithActorForTest(root, parentID, childID, reason string, actor model.IssueOpsActor) (model.IssueOpsChildValidationResult, error) {
	return childValidatorForTest(root).Drop(context.Background(), parentID, childID, reason, &actor)
}
