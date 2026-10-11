package issueopsremote

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	model "issueops/internal/contract/issueops"
)

// memoryRecordStore applies transitions to an in-memory record the way the
// durable store does: a failing transition leaves the record untouched.
type memoryRecordStore struct {
	record model.IssueOpsRecord
	err    error
}

func (s *memoryRecordStore) Update(_ context.Context, _ string, transition RecordTransition) (model.IssueOpsRecord, error) {
	if s.err != nil {
		return model.IssueOpsRecord{}, s.err
	}
	next, err := transition(s.record)
	if err != nil {
		return next, err
	}
	s.record = next
	return next, nil
}

var receiptClock = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 6, time.FixedZone("kst", 9*3600)) }

const receiptStamp = "2026-01-01T18:04:05.000000006Z"

func TestCompletionReceiptsStampUTCAndKeepEarliestClose(t *testing.T) {
	store := &memoryRecordStore{}
	receipts := NewCompletionReceipts(store, receiptClock)
	got, err := receipts.Closed(t.Context(), "c")
	if err != nil || got.RemoteCompletion == nil || got.RemoteCompletion.IssueClosedAt != receiptStamp || got.UpdatedAt != receiptStamp {
		t.Fatalf("closed=%+v err=%v", got, err)
	}
	later := NewCompletionReceipts(store, func() time.Time { return receiptClock().Add(time.Hour) })
	got, err = later.Reflected(t.Context(), "c")
	if err != nil || got.RemoteCompletion.ReflectedAt == "" || got.RemoteCompletion.IssueClosedAt != receiptStamp {
		t.Fatalf("reflected must preserve the close receipt: %+v err=%v", got.RemoteCompletion, err)
	}
	got, err = later.Closed(t.Context(), "c")
	if err != nil || got.RemoteCompletion.IssueClosedAt != receiptStamp {
		t.Fatalf("second close must not move the receipt: %+v err=%v", got.RemoteCompletion, err)
	}
}

func TestCompletionReceiptsReportStoreFailureAsNotOK(t *testing.T) {
	boom := errors.New("store down")
	got, err := NewCompletionReceipts(&memoryRecordStore{err: boom}, receiptClock).Reflected(t.Context(), "c")
	if !errors.Is(err, boom) || got.OK || got.RemoteCompletion != nil {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func issueCreateRequest() model.IssueOpsIssueCreateIntentRequest {
	return model.IssueOpsIssueCreateIntentRequest{
		OperationID: strings.Repeat("b", 32), Provider: "github", ProjectAuthority: "github.com/acme/app",
		Title: "title", BodySHA256: strings.Repeat("a", 64), StartedAt: "2026-01-01T00:00:00Z",
	}
}

func TestIssueCreateIntentsLifecycleBindsProjectAuthority(t *testing.T) {
	store := &memoryRecordStore{}
	intents := NewIssueCreateIntents(store, receiptClock)
	rec, err := intents.Begin(t.Context(), "c", issueCreateRequest())
	if err != nil || rec.IssueCreateIntent == nil || rec.IssueCreateIntent.Status != model.IssueCreateIntentPending || rec.UpdatedAt != receiptStamp {
		t.Fatalf("begin=%+v err=%v", rec, err)
	}
	if _, err := intents.Begin(t.Context(), "c", issueCreateRequest()); err == nil {
		t.Fatal("a pending intent must not be begun twice")
	}

	// Another project's issue must not be accepted as the sealed project's.
	_, err = intents.Complete(t.Context(), "c", "https://github.com/other/repo/issues/9", "now")
	if err == nil || !strings.Contains(err.Error(), "does not match sealed authority") {
		t.Fatalf("foreign project accepted: %v", err)
	}
	_, err = intents.Outcome(t.Context(), "c", model.IssueOpsIssueCreateOutcome{Status: model.IssueCreateIntentURLObserved, CanonicalURL: "https://github.com/other/repo/issues/9", ObservedAt: "now"})
	if err == nil {
		t.Fatal("outcome for a foreign project accepted")
	}

	rec, err = intents.Outcome(t.Context(), "c", model.IssueOpsIssueCreateOutcome{Status: model.IssueCreateIntentURLObserved, CanonicalURL: "https://github.com/acme/app/issues/9", Failure: "receipt pending", ObservedAt: "t1"})
	if err != nil || rec.IssueCreateIntent.CanonicalURL != "https://github.com/acme/app/issues/9" {
		t.Fatalf("outcome=%+v err=%v", rec.IssueCreateIntent, err)
	}
	rec, err = intents.Complete(t.Context(), "c", "https://github.com/acme/app/issues/9", "t2")
	if err != nil || rec.IssueCreateIntent.Status != model.IssueCreateIntentCompleted {
		t.Fatalf("complete=%+v err=%v", rec.IssueCreateIntent, err)
	}
	if _, err := intents.Complete(t.Context(), "c", "https://github.com/acme/app/issues/9", "t3"); err == nil {
		t.Fatal("completed intent must not complete again")
	}
}

func TestIssueCreateIntentsOutcomeWithoutIntentFails(t *testing.T) {
	intents := NewIssueCreateIntents(&memoryRecordStore{}, receiptClock)
	_, err := intents.Outcome(t.Context(), "c", model.IssueOpsIssueCreateOutcome{Status: model.IssueCreateIntentNotInvoked, Failure: "x", ObservedAt: "t"})
	if err == nil || err.Error() != "no pending issue create intent" {
		t.Fatalf("err=%v", err)
	}
}
