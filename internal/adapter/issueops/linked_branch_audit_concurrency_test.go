package issueops

import (
	"context"
	"errors"
	"io/fs"
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
	linkedbranch "issueops/internal/domain/issueopslinkedbranch"
)

func TestLinkedBranchAuditPreservesConcurrentCycleChanges(t *testing.T) {
	for _, name := range []string{"metadata", "applying fence", "changed target", "deleted cycle"} {
		t.Run(name, func(t *testing.T) {
			root, record := lbFixture(t)
			var expected model.IssueOpsRecord
			deps := (&lbDeps{}).build()
			deps.ObserveLinkedBranches = func(context.Context, string) (linkedbranch.Observation, error) {
				if name == "deleted cycle" {
					if err := deleteIssueOps(root, record.ID); err != nil {
						t.Fatal(err)
					}
				} else {
					mutateFinishRecord(t, root, record.ID, func(current *model.IssueOpsRecord) {
						switch name {
						case "metadata":
							current.PlanPath = "new-plan.md"
						case "applying fence":
							current.CleanupAbandonFailure = &model.IssueOpsCleanupAbandonFailure{Step: model.CleanupFailureStepApplying}
						case "changed target":
							current.BranchPrepare.BaseSHA = "new-base"
						}
					})
					var err error
					expected, err = ReadIssueOps(root, record.ID)
					if err != nil {
						t.Fatal(err)
					}
				}
				return linkedbranch.Observation{}, nil
			}
			result, err := CleanupLinkedBranch(context.Background(), root, model.CleanupLinkedBranchRequest{ID: record.ID}, deps)
			if err != nil || !result.AlreadyAbsent {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			current, readErr := ReadIssueOps(root, record.ID)
			if name == "metadata" {
				if readErr != nil || !result.AuditRecorded || current.PlanPath != expected.PlanPath || current.LinkedBranchCleanup == nil {
					t.Fatalf("metadata overwritten: record=%+v result=%+v readErr=%v", current, result, readErr)
				}
				return
			}
			if result.AuditRecorded || result.AuditError == "" {
				t.Fatalf("stale audit accepted: %+v", result)
			}
			if name == "deleted cycle" {
				if !errors.Is(readErr, fs.ErrNotExist) {
					t.Fatalf("deleted cycle recreated: record=%+v err=%v", current, readErr)
				}
			} else if readErr != nil || !reflect.DeepEqual(current, expected) {
				t.Fatalf("concurrent state overwritten: want=%+v got=%+v err=%v", expected, current, readErr)
			}
		})
	}
}
