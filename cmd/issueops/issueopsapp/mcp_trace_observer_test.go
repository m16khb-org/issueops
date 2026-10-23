package issueopsapp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"issueops/internal/adapter/outbound/issueopsrecord"
	"issueops/internal/adapter/outbound/sqlstore"
)

func TestMCPTraceBindingObservesDirectSQLSpansAndClearsCorrelation(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	const traceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	t.Setenv("TRACEPARENT", traceparent)
	file, err := os.Create(filepath.Join(t.TempDir(), "spans.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	original := os.Stderr
	os.Stderr = file
	defer func() { os.Stderr = original }()
	deps := issueOpsMCPHTTPDependencies()
	database, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	inherited, _ := issueopsrecord.WithTraceparent(context.Background(), traceparent)
	callbackError := errors.New("force actionable span")
	for _, raw := range []string{traceparent, "", "invalid-private-header"} {
		ctx, valid := deps.BindTrace(inherited, raw)
		if valid != (raw != "invalid-private-header") {
			t.Fatalf("wrong validity for header")
		}
		// A fast failed span is the command's own error and stays silent; a
		// span held past the slow threshold is actionable telemetry.
		slowFailure := func(context.Context) error { time.Sleep(issueOpsSlowSpanThreshold); return callbackError }
		if err := database.WithSpan(ctx, slowFailure); !errors.Is(err, callbackError) {
			t.Fatal(err)
		}
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(file)
	for _, wantTrace := range []string{traceparent[3:35], "", ""} {
		var event issueopsrecord.SpanObservation
		if err := decoder.Decode(&event); err != nil {
			t.Fatalf("missing SQL span event: %v", err)
		}
		if event.TraceID != wantTrace || event.Outcome != sqlstore.SpanOutcomeError {
			t.Fatalf("wrong request event: %+v", event)
		}
	}
}
