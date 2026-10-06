package state

import (
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
)

func TestStateWriteCommitsInsideRequestSpan(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	var observed sqlstore.SpanObservation
	ctx := sqlstore.WithSpanObserver(t.Context(), func(observation sqlstore.SpanObservation) { observed = observation })
	if _, err := NewService().Write(ctx, "request-span", "body"); err != nil {
		t.Fatal(err)
	}
	if observed.CommitCount != 1 || observed.CommitCoverage != sqlstore.SpanCommitCoverageComplete {
		t.Fatalf("state write commit was not attributed to the request span: %+v", observed)
	}
}
