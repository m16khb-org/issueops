package issueopsrecord

import (
	"context"
	"io"
	"testing"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
)

func BenchmarkStoreSpanObserverOverhead(b *testing.B) {
	observers := []struct {
		name     string
		observer Observer
	}{
		{"nil", nil},
		{"noop", ObserverFunc(func(SpanObservation) {})},
		{"json_actionable", NewJSONLineObserver(io.Discard, time.Nanosecond)},
	}
	for _, candidate := range observers {
		b.Run(candidate.name, func(b *testing.B) {
			id := "io-bench01"
			stateRoot := seedObservedRecord(b, id)
			store := Store{Scope: "bench", Observer: candidate.observer}
			mutate := func(record issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, bool, error) {
				record.Branch = "bench"
				return record, true, nil
			}
			b.ReportAllocs()
			for b.Loop() {
				if _, err := store.Update(context.Background(), stateRoot, id, mutate); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
