package issueops

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	model "issueops/internal/contract/issueops"
	publicationcontract "issueops/internal/contract/issueopspublication"
	"issueops/internal/port"
)

func TestFinishAttemptBlocksLegacyWriterEntrypoints(t *testing.T) {
	for _, operation := range []string{"stale write", "delete", "span", "abandon bypass", "execution write", "raw execution write", "parent pair write", "child pair write", "publication"} {
		t.Run(operation, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "state")
			stale := model.IssueOpsRecord{SchemaVersion: 1, ID: "io-finish-fence", Phase: model.IssueOpsPhaseDone}
			armed := stale
			armed.CleanupFinishAttempt = &model.IssueOpsCleanupFinishAttempt{Token: strings.Repeat("a", 64), StartedAt: "2026-09-29T00:00:00Z"}
			_, raw, err := encodeIssueOpsRecord(armed)
			if err != nil {
				t.Fatal(err)
			}
			db, err := sqlstore.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Put(issueOpsBucket, armed.ID, raw); err != nil {
				t.Fatal(err)
			}
			related := []byte(`{"evidence":"keep"}`)
			if err := db.Put(artifactStageBucket, armed.ID, related); err != nil {
				t.Fatal(err)
			}
			if err := db.Put(externalIntentBucket, "pending-finish", related); err != nil {
				t.Fatal(err)
			}
			called := false
			callback := func(context.Context) error { called = true; return nil }
			switch operation {
			case "publication":
				_, err = (RemotePublicationStore{StateRoot: root}).Persist(context.Background(), stale, publicationcontract.IntentMutation{OperationID: "pending-finish", Delete: true})
			case "stale write":
				_, err = writeIssueOps(root, stale)
			case "execution write":
				_, err = persistExecutionTransition(root, stale, nil)
			case "raw execution write":
				_, err = persistExecutionTransitionWithRawCAS(root, stale, []port.ExpectedRecord{{Bucket: issueOpsBucket, ID: armed.ID, Data: raw}}, nil)
			case "parent pair write":
				_, _, err = (ChildCycleStore{CycleRecordStore{StateRoot: root}}).SavePair(context.Background(), stale, model.IssueOpsRecord{SchemaVersion: 1, ID: "io-new-child", Phase: model.IssueOpsPhaseProblem})
			case "child pair write":
				_, _, err = (ChildCycleStore{CycleRecordStore{StateRoot: root}}).SavePair(context.Background(), model.IssueOpsRecord{SchemaVersion: 1, ID: "io-new-parent", Phase: model.IssueOpsPhaseProblem}, stale)
			case "delete":
				err = deleteIssueOps(root, armed.ID)
			case "span":
				err = withIssueOpsLock(context.Background(), root, armed.ID, callback)
			case "abandon bypass":
				err = withCleanupAbandonLock(context.Background(), root, armed.ID, callback)
			}
			if err == nil || !strings.Contains(err.Error(), "cleanup finish") || called {
				t.Fatalf("armed record reached legacy writer: called=%v err=%v", called, err)
			}
			if operation == "child pair write" || operation == "parent pair write" {
				for _, id := range []string{"io-new-parent", "io-new-child"} {
					if _, found, err := db.Get(issueOpsBucket, id); err != nil || found {
						t.Fatalf("partial pair: %s found=%v err=%v", id, found, err)
					}
				}
			}
			if got, found, err := db.Get(externalIntentBucket, "pending-finish"); err != nil || !found || !bytes.Equal(got, related) {
				t.Fatalf("publication intent changed: %v", err)
			}
			got, exists, err := db.Get(issueOpsBucket, armed.ID)
			if err != nil || !exists || !bytes.Equal(got, raw) {
				t.Fatalf("record changed: exists=%v err=%v", exists, err)
			}
			got, exists, err = db.Get(artifactStageBucket, armed.ID)
			if err != nil || !exists || !bytes.Equal(got, related) {
				t.Fatalf("related row changed: exists=%v err=%v", exists, err)
			}
		})
	}
}

func TestOrdinaryWritersCannotRestoreDrainedFinishAttempt(t *testing.T) {
	for _, operation := range []string{"write", "execution", "raw execution", "parent pair", "child pair", "publication"} {
		t.Run(operation, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "state")
			current := model.IssueOpsRecord{SchemaVersion: 1, ID: "io-drained", Phase: model.IssueOpsPhaseDone}
			_, raw, err := encodeIssueOpsRecord(current)
			if err != nil {
				t.Fatal(err)
			}
			db, err := sqlstore.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Put(issueOpsBucket, current.ID, raw); err != nil {
				t.Fatal(err)
			}
			stale := current
			stale.CleanupFinishAttempt = &model.IssueOpsCleanupFinishAttempt{Token: strings.Repeat("a", 64), StartedAt: "2026-09-29T00:00:00Z"}
			other := model.IssueOpsRecord{SchemaVersion: 1, ID: "io-other", Phase: model.IssueOpsPhaseProblem}
			switch operation {
			case "publication":
				_, err = (RemotePublicationStore{StateRoot: root}).Persist(context.Background(), stale, publicationcontract.IntentMutation{OperationID: "pending-finish", Delete: true})
			case "write":
				_, err = writeIssueOps(root, stale)
			case "execution":
				_, err = persistExecutionTransition(root, stale, nil)
			case "raw execution":
				_, err = persistExecutionTransitionWithRawCAS(root, stale, []port.ExpectedRecord{{Bucket: issueOpsBucket, ID: current.ID, Data: raw}}, nil)
			case "parent pair":
				_, _, err = (ChildCycleStore{CycleRecordStore{StateRoot: root}}).SavePair(context.Background(), stale, other)
			case "child pair":
				_, _, err = (ChildCycleStore{CycleRecordStore{StateRoot: root}}).SavePair(context.Background(), other, stale)
			}
			if err == nil || !strings.Contains(err.Error(), "cleanup finish") {
				t.Fatalf("restored stale attempt: %v", err)
			}
			got, exists, err := db.Get(issueOpsBucket, current.ID)
			if err != nil || !exists || !bytes.Equal(got, raw) {
				t.Fatalf("changed current: %v", err)
			}
			if _, exists, err := db.Get(issueOpsBucket, other.ID); err != nil || exists {
				t.Fatalf("partially wrote pair: exists=%v err=%v", exists, err)
			}
		})
	}
}
