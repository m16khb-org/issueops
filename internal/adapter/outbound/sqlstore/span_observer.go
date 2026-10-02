package sqlstore

import (
	"context"
	"sync/atomic"
	"time"
)

const (
	SpanOutcomeSuccess  = "success"
	SpanOutcomeError    = "error"
	SpanOutcomeCanceled = "canceled"
	SpanOutcomeNested   = "nested"
)

const (
	// SpanCommitCoverageComplete는 lock 보유 동안 이 핸들의 모든 data commit이
	// span context를 통해 관측됐다는 뜻이다. 이때만 Commit 합계(0 포함)를 보고한다.
	SpanCommitCoverageComplete = "complete"
	// SpanCommitCoverageUnknown은 span context에 귀속할 수 없는 write(autocommit
	// Put/Delete, span 밖 context의 Apply, 같은 핸들의 동시 writer)가 있었다는 뜻이다.
	SpanCommitCoverageUnknown = "unknown"
)

// SpanObservation은 span 하나의 단계 지연이다. 단계는 서로 겹치므로 더하지 않는다.
//   - Wait: 진입부터 SQLite span lock 획득까지(local gate 대기 포함). 미획득이면 진입부터 반환까지.
//   - Hold: lock 획득부터 lock rollback 완료까지. 미획득이면 0이고 Acquired=false다.
//   - Callback: fn 진입부터 반환까지의 inclusive 시간(commit 포함). 미진입이면 nil.
//   - Commit: span context로 실행한 data tx.Commit 호출 시간의 합. callback의 부분집합이며
//     병렬 commit의 합은 wall time이 아니다. coverage가 unknown이면 nil.
//   - Total: 진입부터 gate 반환 완료까지. observer 실행 시간은 어느 단계에도 들어가지 않는다.
type SpanObservation struct {
	Outcome        string
	Contended      bool
	Acquired       bool
	Wait           time.Duration
	Hold           time.Duration
	Callback       *time.Duration
	Commit         *time.Duration
	Total          time.Duration
	CommitCount    int
	CommitCoverage string
}

type SpanObserver func(SpanObservation)

type spanObserverKey struct{}

func WithSpanObserver(ctx context.Context, observer SpanObserver) context.Context {
	if ctx == nil || observer == nil {
		return ctx
	}
	return context.WithValue(ctx, spanObserverKey{}, observer)
}

func spanObserver(ctx context.Context) SpanObserver {
	if ctx == nil {
		return nil
	}
	observer, _ := ctx.Value(spanObserverKey{}).(SpanObserver)
	return observer
}

type spanRunKey struct{}

// spanRun은 WithSpan 호출 하나의 context-local 누적기다. 전역 current-span 없이
// parent 사슬로 다른 root의 중첩 span을 건너 자기 핸들의 span을 찾는다.
type spanRun struct {
	db     *DB
	parent *spanRun
	now    func() time.Time

	active      atomic.Bool
	commitNanos atomic.Int64
	commitCount atomic.Int64

	contended          bool
	acquired           bool
	callbackEntered    bool
	unattributedWrites bool
	unattributedBase   uint64
	lockAcquired       time.Time
	lockReleased       time.Time
	gateReleased       time.Time
	callback           time.Duration
}

func activeSpanRun(ctx context.Context, d *DB) *spanRun {
	run, _ := ctx.Value(spanRunKey{}).(*spanRun)
	for ; run != nil; run = run.parent {
		if run.db == d {
			if run.active.Load() {
				return run
			}
			return nil
		}
	}
	return nil
}

func (run *spanRun) invoke(ctx context.Context, fn func(context.Context) error) error {
	started := run.now()
	run.callbackEntered = true
	defer func() { run.callback = run.now().Sub(started) }()
	return fn(ctx)
}

func (run *spanRun) observation(started, ended time.Time, outcome string) SpanObservation {
	observation := SpanObservation{
		Outcome:     outcome,
		Contended:   run.contended,
		Acquired:    run.acquired,
		Total:       ended.Sub(started),
		CommitCount: int(run.commitCount.Load()),
	}
	if run.acquired {
		observation.Wait = run.lockAcquired.Sub(started)
		observation.Hold = run.lockReleased.Sub(run.lockAcquired)
	} else {
		observation.Wait = observation.Total
	}
	if run.callbackEntered {
		callback := run.callback
		observation.Callback = &callback
	}
	if run.unattributedWrites {
		observation.CommitCoverage = SpanCommitCoverageUnknown
	} else {
		commit := time.Duration(run.commitNanos.Load())
		observation.Commit = &commit
		observation.CommitCoverage = SpanCommitCoverageComplete
	}
	return observation
}
