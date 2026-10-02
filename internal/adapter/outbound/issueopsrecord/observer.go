package issueopsrecord

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"issueops/internal/adapter/outbound/sqlstore"
	tracecontract "issueops/internal/contract/trace"
)

const defaultSlowSpanThreshold = 100 * time.Millisecond

type SpanObservation struct {
	Event       string `json:"event"`
	GeneratedAt string `json:"generated_at"`
	Operation   string `json:"operation"`
	Outcome     string `json:"outcome"`
	Contended   bool   `json:"contended"`
	WaitMS      int64  `json:"wait_ms"`
	HoldMS      int64  `json:"hold_ms"`
	// 아래 단계는 서로 겹치므로 더하지 않는다. callback_ms는 commit을 포함하고,
	// null은 미진입/미관측(unknown)이며 0은 관측된 값이다.
	Acquired       bool   `json:"acquired"`
	CallbackMS     *int64 `json:"callback_ms"`
	CommitMS       *int64 `json:"commit_ms"`
	TotalMS        int64  `json:"total_ms"`
	CommitCount    int    `json:"commit_count"`
	CommitCoverage string `json:"commit_coverage"`
	TraceID        string `json:"trace_id,omitempty"`
	ParentID       string `json:"parent_id,omitempty"`
	TraceFlags     string `json:"trace_flags,omitempty"`
}

type Observer interface {
	Observe(SpanObservation)
}

type ObserverFunc func(SpanObservation)

func (observe ObserverFunc) Observe(observation SpanObservation) {
	observe(observation)
}

type jsonLineObserver struct {
	writer        io.Writer
	slowThreshold time.Duration
	mutex         sync.Mutex
}

func NewJSONLineObserver(writer io.Writer, slowThreshold time.Duration) Observer {
	if slowThreshold <= 0 {
		slowThreshold = defaultSlowSpanThreshold
	}
	return &jsonLineObserver{writer: writer, slowThreshold: slowThreshold}
}

func (observer *jsonLineObserver) Observe(observation SpanObservation) {
	if observer == nil || observer.writer == nil {
		return
	}
	thresholdMS := observer.slowThreshold.Milliseconds()
	if observation.Outcome == sqlstore.SpanOutcomeSuccess &&
		!observation.Contended &&
		observation.WaitMS < thresholdMS &&
		observation.HoldMS < thresholdMS {
		return
	}
	observation.Event = "issueops_record_span"
	if strings.TrimSpace(observation.GeneratedAt) == "" {
		observation.GeneratedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	line, err := json.Marshal(observation)
	if err != nil {
		return
	}
	observer.mutex.Lock()
	defer observer.mutex.Unlock()
	if _, err := observer.writer.Write(append(line, '\n')); err != nil && observer.writer != os.Stderr {
		_, _ = fmt.Fprintf(os.Stderr, "issueops span observability write failed: %v\n", err)
	}
}

// ObserveSpans arms the store observer for every sqlstore span opened with the
// returned context, including spans the caller opens directly on a DB handle.
// The request correlation already bound to ctx is captured at this point, so
// bind the traceparent before calling ObserveSpans.
func (store Store) ObserveSpans(ctx context.Context, operation string) context.Context {
	if store.Observer == nil || ctx == nil {
		return ctx
	}
	scope := strings.TrimSpace(store.Scope)
	if scope == "" {
		scope = "issueops"
	}
	correlation, _ := ctx.Value(recordTraceContextKey{}).(tracecontract.TraceContext)
	return sqlstore.WithSpanObserver(ctx, func(observation sqlstore.SpanObservation) {
		store.Observer.Observe(SpanObservation{
			Operation:      scope + "." + operation,
			Outcome:        observation.Outcome,
			Contended:      observation.Contended,
			WaitMS:         max(0, observation.Wait.Milliseconds()),
			HoldMS:         max(0, observation.Hold.Milliseconds()),
			Acquired:       observation.Acquired,
			CallbackMS:     optionalMilliseconds(observation.Callback),
			CommitMS:       optionalMilliseconds(observation.Commit),
			TotalMS:        max(0, observation.Total.Milliseconds()),
			CommitCount:    observation.CommitCount,
			CommitCoverage: observation.CommitCoverage,
			TraceID:        correlation.TraceID,
			ParentID:       correlation.ParentID,
			TraceFlags:     correlation.Flags,
		})
	})
}

func optionalMilliseconds(duration *time.Duration) *int64 {
	if duration == nil {
		return nil
	}
	milliseconds := max(0, duration.Milliseconds())
	return &milliseconds
}
