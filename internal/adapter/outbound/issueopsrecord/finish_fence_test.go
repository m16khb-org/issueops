package issueopsrecord

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	model "issueops/internal/contract/issueops"
)

func TestArmedFinishRefusesOrdinaryRecordMutations(t *testing.T) {
	for _, operation := range []string{"update", "related update", "delete", "unchanged delete"} {
		t.Run(operation, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "state")
			record := model.IssueOpsRecord{SchemaVersion: 1, ID: "io-finish-fence", Phase: model.IssueOpsPhaseDone, CleanupFinishAttempt: &model.IssueOpsCleanupFinishAttempt{Token: strings.Repeat("a", 64), StartedAt: "2026-09-29T00:00:00Z"}}
			raw, err := Encode(record)
			if err != nil {
				t.Fatal(err)
			}
			db, err := sqlstore.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Put(Bucket(), record.ID, raw); err != nil {
				t.Fatal(err)
			}
			related := []byte(`{"evidence":"keep"}`)
			if err := db.Put("artifact_stage_v1", record.ID, related); err != nil {
				t.Fatal(err)
			}
			store := Store{}
			ctx := context.Background()
			called := false
			switch operation {
			case "update":
				_, err = store.Update(ctx, root, record.ID, func(r model.IssueOpsRecord) (model.IssueOpsRecord, bool, error) {
					called = true
					r.Branch = "replacement"
					return r, true, nil
				})
			case "related update":
				_, err = store.UpdateRelated(ctx, root, record.ID, "artifact_stage_v1", func(r model.IssueOpsRecord, b []byte, exists bool) ([]byte, bool, error) {
					called = true
					return []byte(`{"evidence":"replaced"}`), false, nil
				})
			case "delete":
				err = store.Delete(ctx, root, record.ID, "artifact_stage_v1")
			case "unchanged delete":
				err = store.DeleteIfUnchanged(ctx, root, record.ID, record, "artifact_stage_v1")
			}
			if err == nil || !strings.Contains(err.Error(), "cleanup finish") || called {
				t.Fatalf("armed record reached ordinary mutation: called=%v err=%v", called, err)
			}
			got, exists, err := db.Get(Bucket(), record.ID)
			if err != nil || !exists || !bytes.Equal(got, raw) {
				t.Fatalf("record changed: exists=%v err=%v", exists, err)
			}
			got, exists, err = db.Get("artifact_stage_v1", record.ID)
			if err != nil || !exists || !bytes.Equal(got, related) {
				t.Fatalf("related row changed: exists=%v err=%v", exists, err)
			}
		})
	}
}

func TestUpdateCannotRestoreDrainedFinishAttempt(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	record := model.IssueOpsRecord{SchemaVersion: 1, ID: "io-drained", Phase: model.IssueOpsPhaseDone}
	raw, err := Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(Bucket(), record.ID, raw); err != nil {
		t.Fatal(err)
	}
	_, err = (Store{}).Update(context.Background(), root, record.ID, func(r model.IssueOpsRecord) (model.IssueOpsRecord, bool, error) {
		r.CleanupFinishAttempt = &model.IssueOpsCleanupFinishAttempt{Token: strings.Repeat("a", 64), StartedAt: "2026-09-29T00:00:00Z"}
		return r, true, nil
	})
	if err == nil || !strings.Contains(err.Error(), "cleanup finish") {
		t.Fatalf("restored stale attempt: %v", err)
	}
	got, exists, err := db.Get(Bucket(), record.ID)
	if err != nil || !exists || !bytes.Equal(got, raw) {
		t.Fatalf("changed current: %v", err)
	}
}
