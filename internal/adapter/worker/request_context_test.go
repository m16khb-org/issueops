package worker

import (
	"context"
	"errors"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
)

func TestWorkerEnqueueCommitsInsideRequestSpan(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	var observed sqlstore.SpanObservation
	ctx := sqlstore.WithSpanObserver(t.Context(), func(observation sqlstore.SpanObservation) { observed = observation })
	if _, err := testWorkerService().Enqueue(ctx, "request-span", "payload"); err != nil {
		t.Fatal(err)
	}
	if observed.CommitCount != 1 || observed.CommitCoverage != sqlstore.SpanCommitCoverageComplete {
		t.Fatalf("worker write commit was not attributed to the request span: %+v", observed)
	}
}

func TestWorkerEnqueueRefusesCancelledRequest(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := testWorkerService().Enqueue(ctx, "cancelled", "payload"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled enqueue err = %v, want context.Canceled", err)
	}
	listed, err := ListWorkerJobs()
	if err != nil || len(listed.Jobs) != 0 {
		t.Fatalf("cancelled enqueue persisted jobs: %+v err=%v", listed.Jobs, err)
	}
}
