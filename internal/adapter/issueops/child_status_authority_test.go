package issueops

import (
	"context"
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

type childStatusScanTransition struct {
	ChildCycleStore
	change func()
}

func (s childStatusScanTransition) Scan() ([]model.IssueOpsRecord, error) {
	records, err := s.ChildCycleStore.Scan()
	if err == nil {
		s.change()
	}
	return records, err
}

func TestChildIndexRepairReauthorizesParentAfterScan(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	parent := createDelegationReadyParentForTest(t, root)
	started, err := startIssueOpsChildForTest(root, parent, model.IssueOpsChildStartRequest{ParentID: parent.ID, Branch: "124-repair-child", TaskScope: "repair with current authority", AcceptanceCriteria: []string{"stale actor cannot repair"}})
	if err != nil {
		t.Fatal(err)
	}
	parent, err = ReadIssueOps(root, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	parent.ChildCycles = nil
	parent = writeIssueOpsRecordForDelegationTest(t, root, parent)
	service := childStatusServiceForTest(root)
	var transitioned model.IssueOpsRecord
	service.Records = childStatusScanTransition{ChildCycleStore: ChildCycleStore{CycleRecordStore: CycleRecordStore{StateRoot: root}}, change: func() {
		current, err := ReadIssueOps(root, parent.ID)
		if err != nil {
			t.Fatal(err)
		}
		current.Execution.Lease.Holder.SessionID = "new-session"
		transitioned = writeIssueOpsRecordForDelegationTest(t, root, current)
	}}
	actor := issueOpsActorForTest(parent.WorktreePath)
	result, err := service.Status(context.Background(), parent.ID, true, &actor)
	if err == nil || result.Repaired {
		t.Fatalf("stale repair allowed: %+v %v", result, err)
	}
	after, err := ReadIssueOps(root, parent.ID)
	if err != nil || !reflect.DeepEqual(after, transitioned) {
		t.Fatalf("refused repair changed parent: %v", err)
	}
	service = childStatusServiceForTest(root)
	actor.SessionID = "new-session"
	result, err = service.Status(context.Background(), parent.ID, true, &actor)
	if err != nil || !result.Repaired || !reflect.DeepEqual(result.RepairAppended, []string{started.Child.ID}) {
		t.Fatalf("new holder cannot repair: %+v %v", result, err)
	}
}
