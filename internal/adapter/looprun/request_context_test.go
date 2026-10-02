package looprun

import (
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	loopcontract "issueops/internal/contract/looprun"
)

func TestLoopStartCommitsInsideRequestSpan(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	var observed sqlstore.SpanObservation
	ctx := sqlstore.WithSpanObserver(t.Context(), func(observation sqlstore.SpanObservation) { observed = observation })
	if _, err := testLoopService().Start(ctx, loopcontract.StartLoopRequest{Repo: t.TempDir(), Name: "request-span", Goal: "attribute the commit"}); err != nil {
		t.Fatal(err)
	}
	if observed.CommitCount != 1 || observed.CommitCoverage != sqlstore.SpanCommitCoverageComplete {
		t.Fatalf("loop write commit was not attributed to the request span: %+v", observed)
	}
}
