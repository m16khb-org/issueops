package state

import (
	"context"
	"errors"
	"io/fs"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	statecontract "issueops/internal/contract/state"
)

func TestStateWriteCommitsInsideRequestSpan(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	var observed sqlstore.SpanObservation
	ctx := sqlstore.WithSpanObserver(t.Context(), func(observation sqlstore.SpanObservation) { observed = observation })
	if _, err := StateWrite(ctx, "request-span", "body"); err != nil {
		t.Fatal(err)
	}
	if observed.CommitCount != 1 || observed.CommitCoverage != sqlstore.SpanCommitCoverageComplete {
		t.Fatalf("state write commit was not attributed to the request span: %+v", observed)
	}
}

func TestStateUpdateCancelledInsideSpanPersistsNothing(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	ctx, cancel := context.WithCancel(t.Context())
	_, err := StateUpdate(ctx, "cancelled", func(statecontract.RecordEnvelope) (statecontract.RecordEnvelope, error) {
		cancel()
		return statecontract.RecordEnvelope{SchemaVersion: statecontract.SchemaVersion, Key: "cancelled", Content: "x", Bytes: 1, UpdatedAt: "2026-10-02T00:00:00Z"}, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled update err = %v, want context.Canceled", err)
	}
	if _, err := StateRead("cancelled"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("cancelled update persisted a record: %v", err)
	}
}
