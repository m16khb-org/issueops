package issueops

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	model "issueops/internal/contract/issueops"
)

func TestFinishRecordsBindArmToObservedRevision(t *testing.T) {
	root := t.TempDir()
	record, err := writeIssueOps(root, model.IssueOpsRecord{SchemaVersion: 1, ID: "io-finish-records", Phase: model.IssueOpsPhaseDone})
	if err != nil {
		t.Fatal(err)
	}
	store := FinishRecordStore{StateRoot: root}
	snapshot, err := store.Load(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	record.Branch = "replacement"
	if _, err := writeIssueOps(root, record); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Arm(context.Background(), snapshot, finishRecordsAttempt("a")); err == nil {
		t.Fatal("stale observation armed a changed record")
	}
	current, err := store.Load(context.Background(), record.ID)
	if err != nil || current.Record.Branch != "replacement" || current.Record.CleanupFinishAttempt != nil {
		t.Fatalf("current=%+v err=%v", current, err)
	}
	armed, err := store.Arm(context.Background(), current, finishRecordsAttempt("a"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Check(context.Background(), armed); err != nil {
		t.Fatal(err)
	}
	if current.Record.CleanupFinishAttempt != nil {
		t.Fatal("arm mutated its input")
	}
	db, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	changed := armed.Record
	changed.Branch = "out-of-band"
	_, raw, err := encodeIssueOpsRecord(changed)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(issueOpsBucket, changed.ID, raw); err != nil {
		t.Fatal(err)
	}
	if err := store.Check(context.Background(), armed); err == nil {
		t.Fatal("ownership check ignored raw drift")
	}
}

func TestFinishRecordsPreserveReplacementAcrossAllFinalizers(t *testing.T) {
	for _, operation := range []string{"check", "fail", "audit", "delete"} {
		t.Run(operation, func(t *testing.T) {
			root := t.TempDir()
			ctx := context.Background()
			record, err := writeIssueOps(root, model.IssueOpsRecord{SchemaVersion: 1, ID: "io-finish-replaced", Phase: model.IssueOpsPhaseDone})
			if err != nil {
				t.Fatal(err)
			}
			store := FinishRecordStore{StateRoot: root}
			snapshot, err := store.Load(ctx, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			old, err := store.Arm(ctx, snapshot, finishRecordsAttempt("a"))
			if err != nil {
				t.Fatal(err)
			}
			replacement, err := store.Arm(ctx, old, finishRecordsAttempt("b"))
			if err != nil {
				t.Fatal(err)
			}
			db, err := sqlstore.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			stage := []byte(`{"keep":"replacement"}`)
			if err := db.Put(artifactStageBucket, record.ID, stage); err != nil {
				t.Fatal(err)
			}
			switch operation {
			case "check":
				err = store.Check(ctx, old)
			case "fail":
				_, err = store.Fail(ctx, old, finishRecordsFailure(), true)
			case "audit":
				_, err = store.MarkAuditReflected(ctx, old, "2026-09-29T00:00:01Z")
			case "delete":
				err = store.Delete(ctx, old)
			}
			if err == nil {
				t.Fatal("old owner finalized replacement")
			}
			current, err := store.Load(ctx, record.ID)
			if err != nil || current.Revision != replacement.Revision {
				t.Fatalf("replacement changed: %v", err)
			}
			got, found, err := db.Get(artifactStageBucket, record.ID)
			if err != nil || !found || !bytes.Equal(got, stage) {
				t.Fatalf("stage changed: %v", err)
			}
		})
	}
}

func TestFinishRecordsDrainFailureAuditAndAtomicDelete(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	record, err := writeIssueOps(root, model.IssueOpsRecord{SchemaVersion: 1, ID: "io-finish-drain", Phase: model.IssueOpsPhaseDone, RemoteCompletion: &model.IssueOpsRemoteCompletion{IssueClosedAt: "first"}})
	if err != nil {
		t.Fatal(err)
	}
	store := FinishRecordStore{StateRoot: root}
	snapshot, err := store.Load(ctx, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	armed, err := store.Arm(ctx, snapshot, finishRecordsAttempt("a"))
	if err != nil {
		t.Fatal(err)
	}
	retained, err := store.Fail(ctx, armed, finishRecordsFailure(), false)
	if err != nil {
		t.Fatal(err)
	}
	if retained.Record.CleanupFinishAttempt == nil || armed.Record.CleanupFinishFailure != nil {
		t.Fatal("undrained failure lost ownership or mutated input")
	}
	drained, err := store.Fail(ctx, retained, finishRecordsFailure(), true)
	if err != nil {
		t.Fatal(err)
	}
	if drained.Record.CleanupFinishAttempt != nil {
		t.Fatal("drained failure retained ownership")
	}
	if _, err := writeIssueOps(root, drained.Record); err != nil {
		t.Fatalf("ordinary write after drain: %v", err)
	}
	snapshot, err = store.Load(ctx, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	armed, err = store.Arm(ctx, snapshot, finishRecordsAttempt("b"))
	if err != nil {
		t.Fatal(err)
	}
	reflected, err := store.MarkAuditReflected(ctx, armed, "2026-09-29T00:00:02Z")
	if err != nil {
		t.Fatal(err)
	}
	if reflected.Record.RemoteCompletion.IssueClosedAt != "first" || reflected.Record.RemoteCompletion.ReflectedAt == "" || armed.Record.RemoteCompletion.ReflectedAt != "" {
		t.Fatal("audit projection mutated prior receipt")
	}
	if err := store.Delete(ctx, armed); err == nil {
		t.Fatal("deletion accepted pre-audit revision")
	}
	db, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(artifactStageBucket, record.ID, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, reflected); err != nil {
		t.Fatal(err)
	}
	for _, bucket := range []string{issueOpsBucket, artifactStageBucket} {
		if _, found, err := db.Get(bucket, record.ID); err != nil || found {
			t.Fatalf("delete left %s: found=%v err=%v", bucket, found, err)
		}
	}
}

func finishRecordsAttempt(char string) model.IssueOpsCleanupFinishAttempt {
	return model.IssueOpsCleanupFinishAttempt{Token: strings.Repeat(char, 64), StartedAt: "2026-09-29T00:00:00Z"}
}
func finishRecordsFailure() model.IssueOpsCleanupFinishFailure {
	return model.IssueOpsCleanupFinishFailure{Step: model.CleanupFailureStepWorktreeRemove, Message: "remove failed", At: "2026-09-29T00:00:01Z"}
}

func TestFinishRecordsRejectForgedAttemptProjection(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	record, err := writeIssueOps(root, model.IssueOpsRecord{SchemaVersion: 1, ID: "io-finish-forged", Phase: model.IssueOpsPhaseDone})
	if err != nil {
		t.Fatal(err)
	}
	store := FinishRecordStore{StateRoot: root}
	snapshot, err := store.Load(ctx, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	armed, err := store.Arm(ctx, snapshot, finishRecordsAttempt("a"))
	if err != nil {
		t.Fatal(err)
	}
	forged := armed
	foreign := finishRecordsAttempt("b")
	forged.Record.CleanupFinishAttempt = &foreign
	if err := store.Check(ctx, forged); err == nil {
		t.Fatal("foreign token passed ownership check")
	}
	if _, err := store.Fail(ctx, forged, finishRecordsFailure(), true); err == nil {
		t.Fatal("foreign token cleared ownership")
	}
	if _, err := store.MarkAuditReflected(ctx, forged, "2026-09-29T00:00:01Z"); err == nil {
		t.Fatal("foreign token stamped audit")
	}
	if err := store.Delete(ctx, forged); err == nil {
		t.Fatal("foreign token deleted record")
	}
	current, err := store.Load(ctx, record.ID)
	if err != nil || current.Revision != armed.Revision {
		t.Fatalf("forged projection changed persisted record: %v", err)
	}
}
