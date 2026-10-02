package issueops

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	model "issueops/internal/contract/issueops"
)

type childChangedBeforeParentSpan struct {
	CycleRecordStore
	parentID string
	change   func()
}

func (s childChangedBeforeParentSpan) WithinLock(ctx context.Context, id string, fn func(context.Context) error) error {
	return s.CycleRecordStore.WithinLock(ctx, id, func(spanCtx context.Context) error {
		if id == s.parentID {
			s.change()
		}
		return fn(spanCtx)
	})
}

func TestChildAcceptanceRechecksStateUnderParentSpan(t *testing.T) {
	for _, scenario := range []string{"reopened", "reparented", "cleanup applying", "other repo"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			parent := createDelegationReadyParentForTest(t, root)
			started, err := startIssueOpsChildForTest(root, parent, model.IssueOpsChildStartRequest{ParentID: parent.ID, Branch: "124-changing-child", TaskScope: "accept fresh state", AcceptanceCriteria: []string{"stale observations refused"}})
			if err != nil {
				t.Fatal(err)
			}
			child := started.Child
			child.Phase = model.IssueOpsPhaseDone
			child = writeIssueOpsRecordForDelegationTest(t, root, child)
			parent, err = ReadIssueOps(root, parent.ID)
			if err != nil {
				t.Fatal(err)
			}
			service := childValidatorForTest(root)
			want := ""
			service.Records = childChangedBeforeParentSpan{CycleRecordStore: CycleRecordStore{StateRoot: root}, parentID: parent.ID, change: func() {
				switch scenario {
				case "reopened":
					child.Phase = model.IssueOpsPhaseImplement
					want = "child_not_done"
				case "reparented":
					child.Delegation.ParentCycleID = "io-other-parent"
					want = "child_parent_mismatch"
				case "cleanup applying":
					child.CleanupAbandonFailure = &model.IssueOpsCleanupAbandonFailure{Step: "applying"}
					want = "cleanup abandon apply"
				case "other repo":
					child.Repo = "/other"
					want = "child_repo_mismatch"
				}
				writeIssueOpsRecordForDelegationTest(t, root, child)
			}}
			actor := issueOpsActorForTest(parent.WorktreePath)
			result, err := service.Accept(context.Background(), parent.ID, child.ID, []string{"verified earlier"}, &actor)
			if err == nil || result.OK || !strings.Contains(err.Error(), want) {
				t.Fatalf("stale child accepted: ok=%v err=%v", result.OK, err)
			}
			after, err := ReadIssueOps(root, parent.ID)
			if err != nil || !reflect.DeepEqual(after, parent) {
				t.Fatalf("refusal changed parent: %v", err)
			}
		})
	}
}

func TestChildAcceptancePreservesConcurrentSiblingReceipts(t *testing.T) {
	root := t.TempDir()
	parent := createDelegationReadyParentForTest(t, root)
	var ids []string
	for i := 0; i < 8; i++ {
		started, err := startIssueOpsChildForTest(root, parent, model.IssueOpsChildStartRequest{ParentID: parent.ID, Branch: fmt.Sprintf("124-child-%d", i), TaskScope: "concurrent acceptance", AcceptanceCriteria: []string{"all receipts persist"}})
		if err != nil {
			t.Fatal(err)
		}
		child := started.Child
		child.Phase = model.IssueOpsPhaseDone
		writeIssueOpsRecordForDelegationTest(t, root, child)
		ids = append(ids, child.ID)
	}
	actor := issueOpsActorForTest(parent.WorktreePath)
	var wg sync.WaitGroup
	errs := make(chan error, len(ids))
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, err := childValidatorForTest(root).Accept(context.Background(), parent.ID, id, []string{"checked " + id}, &actor)
			errs <- err
		}(id)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	after, err := ReadIssueOps(root, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.ChildCycles) != 8 {
		t.Fatalf("lost references: %d", len(after.ChildCycles))
	}
	for _, ref := range after.ChildCycles {
		if ref.ValidationVerdict != "accepted" || ref.ValidatedAt == "" || !reflect.DeepEqual(ref.ValidationEvidence, []string{"checked " + ref.CycleID}) {
			t.Fatalf("lost receipt: %+v", ref)
		}
	}
}
