package issueopslease

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	leasecontract "issueops/internal/contract/issueopslease"
	"issueops/internal/port"
	authorityport "issueops/internal/port/authority"
)

// Reconcile canonicalizes in one span, inspects external inventory outside
// it, then commits the receipt through a raw CAS. A grant removed or rotated in
// between changes neither CAS input, so the commit itself must rerun the
// request guard and persist nothing when it fails.
func TestReconcileReceiptRechecksRequestGuardAfterGrantChange(t *testing.T) {
	for name, change := range map[string]port.RecordMutation{
		"removed": {Bucket: "test_grant", ID: "caller", Delete: true},
		"rotated": {Bucket: "test_grant", ID: "caller", Data: []byte("rotated")},
	} {
		t.Run(name, func(t *testing.T) {
			_, seed, db, stateRoot := seededResumeIntentAt(t)
			if err := db.Apply(context.Background(), []port.RecordMutation{{Bucket: "test_grant", ID: "caller", Data: []byte("active")}}); err != nil {
				t.Fatal(err)
			}
			inactive := errors.New("test grant is no longer active")
			checks := 0
			ctx := sqlstore.WithRecordGuard(context.Background(), stateRoot, func(ctx context.Context, reader authorityport.RecordReader) (context.Context, error) {
				checks++
				data, found, err := reader.Get("test_grant", "caller")
				if err != nil {
					return nil, err
				}
				if !found || string(data) != "active" {
					return nil, inactive
				}
				return ctx, nil
			})
			repository := NewReconcileRepository(db, nil)
			id := seed.Progress.Record.ID
			intent, err := repository.Canonicalize(ctx, id)
			if err != nil || checks == 0 {
				t.Fatalf("canonicalize: checks=%d err=%v", checks, err)
			}
			recordBefore, intentBefore := reconcileRows(t, db, id, seed.OperationID)
			if err := db.WithSpan(context.Background(), func(locked context.Context) error {
				return db.Apply(locked, []port.RecordMutation{change})
			}); err != nil {
				t.Fatal(err)
			}
			progress, err := repository.ApplyReceipt(ctx, intent, leasecontract.ReconcileStageReceipt{TerminalPTYID: "recovered"})
			if !errors.Is(err, inactive) {
				t.Fatalf("receipt after grant change: err=%v next_stage=%s, want the guard error", err, progress.NextStage)
			}
			recordAfter, intentAfter := reconcileRows(t, db, id, seed.OperationID)
			if !bytes.Equal(recordBefore, recordAfter) || !bytes.Equal(intentBefore, intentAfter) {
				t.Fatal("rejected receipt still changed the record or intent")
			}
		})
	}
}

func reconcileRows(t *testing.T, db *sqlstore.DB, id, operationID string) ([]byte, []byte) {
	t.Helper()
	record, ok, err := db.Get(recordBucket, id)
	if err != nil || !ok {
		t.Fatalf("record present=%v err=%v", ok, err)
	}
	intent, ok, err := db.Get("external_intent_v1", operationID)
	if err != nil || !ok {
		t.Fatalf("intent present=%v err=%v", ok, err)
	}
	return record, intent
}
