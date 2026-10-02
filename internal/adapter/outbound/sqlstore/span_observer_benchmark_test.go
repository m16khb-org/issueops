package sqlstore

import (
	"context"
	"testing"
)

func BenchmarkSpanObserverOverhead(b *testing.B) {
	observers := []struct {
		name string
		wrap func(context.Context) context.Context
	}{
		{"nil", func(ctx context.Context) context.Context { return ctx }},
		{"noop", func(ctx context.Context) context.Context {
			return WithSpanObserver(ctx, func(SpanObservation) {})
		}},
	}
	callbacks := []struct {
		name string
		run  func(*DB) func(context.Context) error
	}{
		{"empty", func(*DB) func(context.Context) error {
			return func(context.Context) error { return nil }
		}},
		{"one_commit", func(database *DB) func(context.Context) error {
			return func(spanCtx context.Context) error { return applyRow(spanCtx, database, "bench") }
		}},
	}
	for _, callback := range callbacks {
		for _, observer := range observers {
			b.Run(callback.name+"/"+observer.name, func(b *testing.B) {
				database, err := newDB(b.TempDir())
				if err != nil {
					b.Fatal(err)
				}
				b.Cleanup(func() {
					_ = database.data.Close()
					_ = database.span.Close()
				})
				ctx := observer.wrap(context.Background())
				fn := callback.run(database)
				b.ReportAllocs()
				for b.Loop() {
					if err := database.WithSpan(ctx, fn); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
