package issueopslease

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	recordcodec "issueops/internal/adapter/outbound/issueopsrecord"
	leaseapp "issueops/internal/application/issueopslease"
	model "issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
)

func TestIntentWritersRejectFinishInRawOrTypedSnapshot(t *testing.T) {
	for _, source := range []string{"raw", "typed", "both"} {
		for _, operation := range []string{"resume invoking", "resume receipt", "resume failure", "reconcile invoking", "reconcile receipt", "reconcile failure", "reconcile clear"} {
			t.Run(source+"/"+operation, func(t *testing.T) {
				repository, state, db := seededResumeIntent(t)
				attempt := &model.IssueOpsCleanupAttempt{Operation: "finish", Token: strings.Repeat("a", 64), StartedAt: "2026-09-29T00:00:00Z"}
				persisted := state.Progress.Record.Stable
				if source != "typed" {
					persisted.CleanupAttempt = attempt
				}
				raw, err := recordcodec.EncodeLease(persisted)
				if err != nil {
					t.Fatal(err)
				}
				if err := db.Put(recordBucket, persisted.ID, raw); err != nil {
					t.Fatal(err)
				}
				state.RecordRaw = raw
				if source != "raw" {
					state.Progress.Record.Stable.CleanupAttempt = attempt
				}
				reconcile := NewReconcileRepository(db, nil)
				rs := leaseapp.ReconcileIntentState{
					Progress:    leaseapp.ReconcileProgress{Record: state.Progress.Record.Stable, Pending: true, NextStage: state.Stage},
					OperationID: state.OperationID, Stage: state.Stage, InvocationState: state.InvocationState,
					RecordRaw: raw, IntentRaw: state.IntentRaw,
				}
				ctx := context.Background()
				switch operation {
				case "resume invoking":
					_, err = repository.MarkInvoking(ctx, state)
				case "resume receipt":
					_, err = repository.ApplyReceipt(ctx, state, leasecontract.ResumeStageReceipt{TerminalPTYID: "pty-recovered"})
				case "resume failure":
					err = repository.RecordFailure(ctx, state, "unknown", errors.New("external failure"))
				case "reconcile invoking":
					_, err = reconcile.MarkInvoking(ctx, rs)
				case "reconcile receipt":
					_, err = reconcile.ApplyReceipt(ctx, rs, leasecontract.ReconcileStageReceipt{TerminalPTYID: "pty-recovered"})
				case "reconcile failure":
					err = reconcile.RecordFailure(ctx, rs, "unknown", errors.New("external failure"))
				case "reconcile clear":
					_, err = reconcile.ClearIntent(ctx, rs, errors.New("no resource"))
				}
				if err == nil || !strings.Contains(err.Error(), "cleanup finish") {
					t.Fatalf("finish authority ignored: %v", err)
				}
				got, found, err := db.Get(recordBucket, persisted.ID)
				if err != nil || !found || !bytes.Equal(got, raw) {
					t.Fatalf("record changed: %v", err)
				}
				got, found, err = db.Get("external_intent_v1", state.OperationID)
				if err != nil || !found || !bytes.Equal(got, state.IntentRaw) {
					t.Fatalf("intent changed: %v", err)
				}
			})
		}
	}
}
