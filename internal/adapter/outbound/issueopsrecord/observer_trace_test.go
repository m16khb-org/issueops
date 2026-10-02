package issueopsrecord

import (
	"context"
	"testing"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestTraceparentBindingsIsolateRequestsAndClearInheritedContext(t *testing.T) {
	const first = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	const second = "00-11111111111111111111111111111111-2222222222222222-00"
	base, valid := WithTraceparent(context.Background(), first)
	if !valid {
		t.Fatal("valid base traceparent rejected")
	}
	for _, tc := range []struct {
		name, raw, traceID, parentID, flags string
		valid                               bool
	}{
		{"request", second, second[3:35], second[36:52], "00", true},
		{"absent", "", "", "", "", true},
		{"invalid", "private-header-value", "", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, accepted := WithTraceparent(base, tc.raw)
			if accepted != tc.valid {
				t.Fatalf("accepted=%v, want %v", accepted, tc.valid)
			}
			root := seedObservedRecord(t, "io-trace0001")
			var got SpanObservation
			store := Store{Observer: ObserverFunc(func(value SpanObservation) { got = value })}
			if _, err := store.Update(ctx, root, "io-trace0001", func(record issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, bool, error) {
				return record, false, nil
			}); err != nil {
				t.Fatal(err)
			}
			if got.TraceID != tc.traceID || got.ParentID != tc.parentID || got.TraceFlags != tc.flags {
				t.Fatalf("wrong request correlation: %+v", got)
			}
		})
	}
}

func TestConcurrentStoreRequestsKeepTheirOwnTraceparent(t *testing.T) {
	const first = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	const second = "00-11111111111111111111111111111111-2222222222222222-00"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	gate := make(chan struct{})
	results := make(chan error, 2)
	events := make(chan SpanObservation, 2)
	for _, raw := range []string{first, second} {
		bound, valid := WithTraceparent(ctx, raw)
		if !valid {
			t.Fatal("valid request traceparent rejected")
		}
		root := seedObservedRecord(t, "io-trace0001")
		store := Store{Scope: raw[3:35], Observer: ObserverFunc(func(value SpanObservation) { events <- value })}
		go func() {
			select {
			case <-gate:
			case <-ctx.Done():
				results <- ctx.Err()
				return
			}
			_, err := store.Update(bound, root, "io-trace0001", func(record issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, bool, error) {
				record.Branch = "observed"
				return record, true, nil
			})
			results <- err
		}()
	}
	close(gate)
	for range 2 {
		select {
		case err := <-results:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		select {
		case event := <-events:
			if event.Operation != event.TraceID+".update" {
				t.Fatalf("request traces were mixed: %+v", event)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
}
