package issueops

import (
	"context"
	"io/fs"
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

func childStatusServiceForTest(root string) delegationapp.StatusService {
	return delegationapp.StatusService{Records: ChildCycleStore{CycleRecordStore: CycleRecordStore{StateRoot: root}}, Authority: cycleapp.NewMutationAuthority(samePath), Now: time.Now}
}
func IssueOpsChildStatus(root, id string, repair bool) (model.IssueOpsChildStatusResult, error) {
	return childStatusServiceForTest(root).Status(context.Background(), id, repair, nil)
}
func IssueOpsChildStatusWithActor(root, id string, repair bool, actor model.IssueOpsActor) (model.IssueOpsChildStatusResult, error) {
	return childStatusServiceForTest(root).Status(context.Background(), id, repair, &actor)
}

func childValidatorForTest(root string) delegationapp.Validator {
	return delegationapp.Validator{Records: CycleRecordStore{StateRoot: root}, Authority: cycleapp.NewMutationAuthority(samePath), Now: time.Now}
}
func AcceptIssueOpsChildWithActor(root, parentID, childID string, evidence []string, actor model.IssueOpsActor) (model.IssueOpsChildValidationResult, error) {
	return childValidatorForTest(root).Accept(context.Background(), parentID, childID, evidence, &actor)
}
func RejectIssueOpsChildWithActor(root, parentID, childID, reason string, evidence []string, actor model.IssueOpsActor) (model.IssueOpsChildValidationResult, error) {
	return childValidatorForTest(root).Reject(context.Background(), parentID, childID, reason, evidence, &actor)
}
func DropIssueOpsChildWithActor(root, parentID, childID, reason string, actor model.IssueOpsActor) (model.IssueOpsChildValidationResult, error) {
	return childValidatorForTest(root).Drop(context.Background(), parentID, childID, reason, &actor)
}

// Model an absent first observation; the actual SQLite row is re-read under
// the parent span, so existing tests can exercise a child reappearing.
type archivedObservationForTest struct {
	CycleRecordStore
	childID  string
	observed bool
}

func (s *archivedObservationForTest) Load(id string) (model.IssueOpsRecord, error) {
	if id == s.childID && !s.observed {
		s.observed = true
		return model.IssueOpsRecord{}, fs.ErrNotExist
	}
	return s.CycleRecordStore.Load(id)
}
func acceptArchivedIssueOpsChild(root, parentID, childID string, evidence []string, actor *model.IssueOpsActor) (model.IssueOpsChildValidationResult, error) {
	service := childValidatorForTest(root)
	service.Records = &archivedObservationForTest{CycleRecordStore: CycleRecordStore{StateRoot: root}, childID: childID}
	return service.Accept(context.Background(), parentID, childID, evidence, actor)
}
func dropArchivedIssueOpsChild(root, parentID, childID, reason string, actor *model.IssueOpsActor) (model.IssueOpsChildValidationResult, error) {
	service := childValidatorForTest(root)
	service.Records = &archivedObservationForTest{CycleRecordStore: CycleRecordStore{StateRoot: root}, childID: childID}
	return service.Drop(context.Background(), parentID, childID, reason, actor)
}
