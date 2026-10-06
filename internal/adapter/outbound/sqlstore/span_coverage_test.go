package sqlstore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"issueops/internal/port"
)

func seedRow(t *testing.T, database *DB, id string) {
	t.Helper()
	if err := database.Put("span_phase", id, []byte(id)); err != nil {
		t.Fatal(err)
	}
}

func TestSpanCompareAndApplyCommitIsAttributed(t *testing.T) {
	expected := []port.ExpectedRecord{{Bucket: "span_phase", ID: "seed", Data: []byte("seed")}}
	mutations := []port.RecordMutation{{Bucket: "span_phase", ID: "cas", Data: []byte("cas")}}
	for name, write := range map[string]func(context.Context, *DB) error{
		"compare_and_apply": func(ctx context.Context, database *DB) error {
			return database.CompareAndApply(ctx, expected, mutations)
		},
		"compare_and_apply_func": func(ctx context.Context, database *DB) error {
			return database.CompareAndApplyFunc(ctx, expected, func() ([]port.RecordMutation, error) {
				return mutations, nil
			})
		},
	} {
		t.Run(name, func(t *testing.T) {
			database := openTestDB(t)
			seedRow(t, database, "seed")
			// started, lock acquired, callback start, commit start/end,
			// callback end, lock released, gate released.
			clock := newScriptedClock(t, 0, 1, 2, 4, 9, 10, 11, 12)
			ctx, observations := collectSpans(context.Background())
			if err := database.withSpan(ctx, clock.now, func(spanCtx context.Context) error {
				return write(spanCtx, database)
			}); err != nil {
				t.Fatal(err)
			}
			got := onlySpan(t, *observations)
			if got.CommitCount != 1 || got.Commit == nil || *got.Commit != millis(5) ||
				got.CommitCoverage != SpanCommitCoverageComplete {
				t.Fatalf("CAS commit was not attributed to the span: %+v", got)
			}
			clock.requireExhausted()
			if _, found, err := database.Get("span_phase", "cas"); err != nil || !found {
				t.Fatalf("CAS row found=%v err=%v", found, err)
			}
		})
	}
}

func TestSpanAutocommitDeletesAreUnknown(t *testing.T) {
	for name, write := range map[string]func(*DB) error{
		"delete": func(database *DB) error { return database.Delete("span_phase", "seed") },
	} {
		t.Run(name, func(t *testing.T) {
			database := openTestDB(t)
			seedRow(t, database, "seed")
			clock := newScriptedClock(t, 0, 1, 2, 3, 4, 5)
			ctx, observations := collectSpans(context.Background())
			if err := database.withSpan(ctx, clock.now, func(context.Context) error {
				return write(database)
			}); err != nil {
				t.Fatal(err)
			}
			got := onlySpan(t, *observations)
			if got.Commit != nil || got.CommitCount != 0 || got.CommitCoverage != SpanCommitCoverageUnknown {
				t.Fatalf("autocommit delete reported as measured: %+v", got)
			}
			clock.requireExhausted()
			if _, found, err := database.Get("span_phase", "seed"); err != nil || found {
				t.Fatalf("deleted row found=%v err=%v", found, err)
			}
		})
	}
}

// 귀속 불가 write가 span baseline 이전에 시작해 lock 보유 중에 commit되면
// coverage는 unknown이어야 한다. 다른 핸들이 data DB write lock을 잡아 Put이
// 시작 기록 뒤 commit하지 못하게 하고, callback 안에서 그 lock을 풀어 commit이
// 반드시 hold 구간에 일어나게 한다. 두 번째 경우는 write의 끝 기록을 span 해제
// 뒤로 미루므로 baseline의 in-flight 표본이 잡아야 한다. 마지막 경우는 baseline
// 뒤에 시작하므로 write 시작 epoch가 그 write를 잡아야 한다.
func TestSpanCoverageSeesUnattributedWriteInFlightAtBaseline(t *testing.T) {
	for name, timing := range map[string]struct{ delayStart, delayEnd bool }{
		"end_recorded_during_hold":   {false, false},
		"end_recorded_after_release": {false, true},
		"start_recorded_during_hold": {true, true},
	} {
		t.Run(name, func(t *testing.T) {
			database, blocker := openSharedRoot(t)
			hold, err := blocker.data.BeginTx(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			started, committed, resume := make(chan struct{}), make(chan struct{}), make(chan struct{})
			releaseHold := sync.OnceValue(hold.Rollback)
			releaseWrite := sync.OnceFunc(func() { close(resume) })
			putDone, putStopped := make(chan error, 1), make(chan struct{})
			putStarted := false
			t.Cleanup(func() {
				if !putStarted {
					return
				}
				select {
				case <-putStopped:
				case <-time.After(5 * time.Second):
					t.Error("write did not finish after cleanup released its barriers")
				}
			})
			t.Cleanup(releaseWrite)
			t.Cleanup(func() {
				if err := releaseHold(); err != nil {
					t.Error(err)
				}
			})
			waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			wait := func(signal <-chan struct{}) error {
				select {
				case <-signal:
					return nil
				case <-waitCtx.Done():
					return waitCtx.Err()
				}
			}
			database.hooks.unattributedWriteStarted = func() { close(started) }
			database.hooks.unattributedWriteFinished = func() {
				close(committed)
				if timing.delayEnd {
					<-resume
				}
			}
			startWrite := func() error {
				putStarted = true
				go func() {
					defer close(putStopped)
					putDone <- database.Put("span_phase", "in-flight", []byte("v"))
				}()
				return wait(started)
			}
			if !timing.delayStart {
				if err := startWrite(); err != nil {
					t.Fatal(err)
				}
			}

			ctx, observations := collectSpans(waitCtx)
			if err := database.WithSpan(ctx, func(context.Context) error {
				if timing.delayStart {
					if err := startWrite(); err != nil {
						return err
					}
				}
				if err := releaseHold(); err != nil {
					return err
				}
				return wait(committed)
			}); err != nil {
				t.Fatal(err)
			}
			releaseWrite()
			select {
			case err := <-putDone:
				if err != nil {
					t.Fatal(err)
				}
			case <-waitCtx.Done():
				t.Fatal(waitCtx.Err())
			}
			if got := onlySpan(t, *observations); got.Commit != nil || got.CommitCoverage != SpanCommitCoverageUnknown {
				t.Fatalf("write in flight at the baseline committed during the hold but was not reported: %+v", got)
			}
		})
	}
}

func TestSpanCommitChecksCancellationBeforeCommit(t *testing.T) {
	database := openTestDB(t)
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	database.hooks.beforeDataCommit = cancel
	ctx, observations := collectSpans(parent)
	err := database.WithSpan(ctx, func(spanCtx context.Context) error {
		return applyRow(spanCtx, database, "canceled-before-commit")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v want context.Canceled", err)
	}
	if _, found, getErr := database.Get("span_phase", "canceled-before-commit"); getErr != nil || found {
		t.Fatalf("canceled write left row found=%v err=%v", found, getErr)
	}
	got := onlySpan(t, *observations)
	if got.Outcome != SpanOutcomeCanceled || got.CommitCount != 0 ||
		got.Commit == nil || *got.Commit != 0 || got.CommitCoverage != SpanCommitCoverageComplete {
		t.Fatalf("pre-commit cancellation still called Commit: %+v", got)
	}
}

func TestSpanCancellationAfterCommitKeepsData(t *testing.T) {
	database, other := openSharedRoot(t)
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, observations := collectSpans(parent)
	err := database.WithSpan(ctx, func(spanCtx context.Context) error {
		if err := applyRow(spanCtx, database, "committed"); err != nil {
			return err
		}
		cancel()
		return spanCtx.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v want context.Canceled", err)
	}
	if _, found, getErr := other.Get("span_phase", "committed"); getErr != nil || !found {
		t.Fatalf("post-commit cancellation lost data found=%v err=%v", found, getErr)
	}
	got := onlySpan(t, *observations)
	if got.Outcome != SpanOutcomeCanceled || got.CommitCount != 1 || got.CommitCoverage != SpanCommitCoverageComplete {
		t.Fatalf("observation=%+v", got)
	}
}

// Store의 세 write는 각각 단일 mutation Apply(spanContext)다. 각 Apply는 자기
// transaction으로 commit하므로 span lock이 아직 잡혀 있는 동안 다른 핸들에서 보여야 한다.
func TestSpanApplyIsVisibleToAnotherHandleBeforeSpanExit(t *testing.T) {
	database, other := openSharedRoot(t)
	type probe struct {
		found    bool
		lockHeld bool
		err      error
	}
	requests, replies := make(chan string), make(chan probe)
	go func() {
		for id := range requests {
			_, found, err := other.Get("span_phase", id)
			held, lockErr := spanLockHeldElsewhere(other)
			replies <- probe{found: found, lockHeld: held, err: errors.Join(err, lockErr)}
		}
	}()
	defer close(requests)
	ask := func(id string) probe {
		requests <- id
		return <-replies
	}
	ctx, observations := collectSpans(context.Background())
	if err := database.WithSpan(ctx, func(spanCtx context.Context) error {
		if err := applyRow(spanCtx, database, "visible"); err != nil {
			return err
		}
		if got := ask("visible"); got.err != nil || !got.found || !got.lockHeld {
			return fmt.Errorf("upsert inside span: %+v", got)
		}
		if err := database.Apply(spanCtx, []port.RecordMutation{{Bucket: "span_phase", ID: "visible", Delete: true}}); err != nil {
			return err
		}
		if got := ask("visible"); got.err != nil || got.found || !got.lockHeld {
			return fmt.Errorf("delete inside span: %+v", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got := onlySpan(t, *observations); got.CommitCount != 2 || got.CommitCoverage != SpanCommitCoverageComplete {
		t.Fatalf("observation=%+v", got)
	}
}
