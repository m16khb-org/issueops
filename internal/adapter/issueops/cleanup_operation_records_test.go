package issueops

import (
	"context"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestCleanupRecordsBindOperationAndReplacementAcrossFinalizers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	for _, operation := range []model.CleanupOperation{model.CleanupOperationFinish, model.CleanupOperationRemoteBranch} {
		t.Run(string(operation), func(t *testing.T) {
			root := t.TempDir()
			record, err := writeIssueOps(context.Background(), root, model.IssueOpsRecord{SchemaVersion: 1, ID: "io-operation", Phase: model.IssueOpsPhaseDone})
			if err != nil {
				t.Fatal(err)
			}
			store := CleanupRecordStore{StateRoot: root}
			before, err := store.Load(ctx, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			attempt := model.IssueOpsCleanupAttempt{Operation: operation, Token: strings.Repeat("a", 64), StartedAt: "2026-09-29T00:00:00Z"}
			armed, err := store.Arm(ctx, before, attempt)
			if err != nil {
				t.Fatal(err)
			}
			other := model.CleanupOperationRemoteBranch
			if operation == other {
				other = model.CleanupOperationFinish
			}
			foreign := attempt
			foreign.Operation = other
			foreign.Token = strings.Repeat("b", 64)
			if _, err := store.Arm(ctx, armed, foreign); err == nil {
				t.Fatal("foreign operation took over attempt")
			}
			forged := armed
			fake := attempt
			fake.Operation = other
			forged.Record.CleanupAttempt = &fake
			for name, call := range map[string]func() error{
				"check":   func() error { return store.Check(ctx, forged) },
				"release": func() error { _, err := store.Release(ctx, forged, "now"); return err },
				"fail":    func() error { _, err := store.Fail(ctx, forged, finishRecordsFailure(), true); return err },
				"delete":  func() error { return store.Delete(ctx, forged) },
			} {
				if err := call(); err == nil {
					t.Fatalf("forged operation reached %s", name)
				}
			}
			if operation == model.CleanupOperationRemoteBranch {
				if err := store.Delete(ctx, armed); err == nil {
					t.Fatal("remote owner deleted local cycle")
				}
				if _, err := store.Fail(ctx, armed, finishRecordsFailure(), true); err == nil {
					t.Fatal("remote owner applied finish failure")
				}
			} else if _, err := store.Release(ctx, armed, "now"); err == nil {
				t.Fatal("finish owner entered remote release")
			}
			replacement := attempt
			replacement.Token = strings.Repeat("c", 64)
			next, err := store.Arm(ctx, armed, replacement)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.Release(ctx, armed, "now"); err == nil {
				t.Fatal("old token released replacement")
			}
			current, err := store.Load(ctx, record.ID)
			if err != nil || current.Revision != next.Revision {
				t.Fatalf("replacement changed: %v", err)
			}
		})
	}
}
