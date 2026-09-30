package issueops

import (
	"context"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestAbandonRecordFinalizersRejectReplacementAndForeignOperations(t *testing.T) {
	ctx := context.Background()
	for _, operation := range []model.CleanupOperation{model.CleanupOperationFinish, model.CleanupOperationRemoteBranch, model.CleanupOperationAbandon} {
		t.Run(string(operation), func(t *testing.T) {
			root := t.TempDir()
			record, err := writeIssueOps(root, model.IssueOpsRecord{SchemaVersion: 1, ID: "io-abandon-store", Phase: model.IssueOpsPhaseDone})
			if err != nil {
				t.Fatal(err)
			}
			store := CleanupRecordStore{StateRoot: root}
			before, err := store.Load(ctx, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			attempt := model.IssueOpsCleanupAttempt{Operation: operation, Token: strings.Repeat("a", 64), StartedAt: "2026-09-29T00:00:00Z"}
			failure := model.IssueOpsCleanupAbandonFailure{Step: model.CleanupFailureStepApplying, At: attempt.StartedAt}
			var armed model.CleanupSnapshot
			if operation == model.CleanupOperationAbandon {
				if _, err := store.Arm(ctx, before, attempt); err == nil {
					t.Fatal("generic arm bypassed abandon receipt")
				}
				armed, err = store.ArmAbandon(ctx, before, attempt, failure)
			} else {
				armed, err = store.Arm(ctx, before, attempt)
			}
			if err != nil {
				t.Fatal(err)
			}
			if operation != model.CleanupOperationAbandon {
				if _, err := store.FailAbandon(ctx, armed, failure, true); err == nil {
					t.Fatal("foreign owner wrote abandon failure")
				}
				if _, err := store.DeleteAbandoned(ctx, armed); err == nil {
					t.Fatal("foreign owner deleted lifecycle")
				}
				attempt.Operation = model.CleanupOperationAbandon
				attempt.Token = strings.Repeat("b", 64)
				if _, err := store.ArmAbandon(ctx, armed, attempt, failure); err == nil {
					t.Fatal("abandon took over foreign operation")
				}
			} else {
				if _, err := store.Release(ctx, armed, "now"); err == nil {
					t.Fatal("abandon used remote-branch release")
				}
				if err := store.Delete(ctx, armed); err == nil {
					t.Fatal("abandon used finish deletion")
				}
				if _, err := store.Fail(ctx, armed, finishRecordsFailure(), true); err == nil {
					t.Fatal("abandon wrote finish failure")
				}
				attempt.Token = strings.Repeat("b", 64)
				next, err := store.ArmAbandon(ctx, armed, attempt, failure)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := store.FailAbandon(ctx, armed, failure, true); err == nil {
					t.Fatal("stale owner cleared replacement")
				}
				if _, err := store.DeleteAbandoned(ctx, armed); err == nil {
					t.Fatal("stale owner deleted replacement")
				}
				current, err := store.Load(ctx, record.ID)
				if err != nil || current.Revision != next.Revision {
					t.Fatalf("replacement changed: %v", err)
				}
				retained, err := store.FailAbandon(ctx, next, failure, false)
				if err != nil || retained.Record.CleanupAttempt == nil {
					t.Fatalf("undrained attempt lost: %v", err)
				}
				released, err := store.FailAbandon(ctx, retained, model.IssueOpsCleanupAbandonFailure{Step: model.CleanupFailureStepRecordDelete, At: attempt.StartedAt}, true)
				if err != nil || released.Record.CleanupAttempt != nil {
					t.Fatalf("drained attempt remained: %v", err)
				}
			}
		})
	}
}
