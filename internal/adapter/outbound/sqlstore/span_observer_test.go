package sqlstore

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"issueops/internal/port"
)

func TestWithSpanReportsSuccessAndFailureAfterReleasingLock(t *testing.T) {
	database, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var observations []SpanObservation
	ctx := WithSpanObserver(context.Background(), func(observation SpanObservation) {
		observations = append(observations, observation)
	})

	if err := database.WithSpan(ctx, func(context.Context) error {
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("mutation failed")
	if err := database.WithSpan(ctx, func(context.Context) error {
		return sentinel
	}); !errors.Is(err, sentinel) {
		t.Fatalf("callback error = %v", err)
	}
	if len(observations) != 2 {
		t.Fatalf("observations=%d want=2", len(observations))
	}
	if observations[0].Outcome != SpanOutcomeSuccess ||
		observations[0].Wait < 0 ||
		observations[0].Hold < 0 {
		t.Fatalf("success observation = %+v", observations[0])
	}
	if observations[1].Outcome != SpanOutcomeError {
		t.Fatalf("failure observation = %+v", observations[1])
	}

	if err := database.WithSpan(context.Background(), func(context.Context) error {
		return nil
	}); err != nil {
		t.Fatalf("observer ran before releasing span lock: %v", err)
	}
}

// scriptedClock는 span이 읽는 시각을 순서대로 돌려준다. hooks는 해당 순번의
// 읽기 직전에 실행되어 sleep 없이 경계 사이에 사건을 끼워 넣는다.
type scriptedClock struct {
	t       *testing.T
	base    time.Time
	offsets []time.Duration
	hooks   map[int]func()
	reads   int
}

func newScriptedClock(t *testing.T, offsetsMS ...int) *scriptedClock {
	offsets := make([]time.Duration, len(offsetsMS))
	for index, offset := range offsetsMS {
		offsets[index] = time.Duration(offset) * time.Millisecond
	}
	return &scriptedClock{t: t, base: time.Unix(1_800_000_000, 0), offsets: offsets}
}

func (clock *scriptedClock) now() time.Time {
	if clock.reads >= len(clock.offsets) {
		clock.t.Errorf("clock read %d exceeds the %d scripted reads", clock.reads+1, len(clock.offsets))
		return clock.base.Add(clock.offsets[len(clock.offsets)-1])
	}
	if hook := clock.hooks[clock.reads]; hook != nil {
		hook()
	}
	value := clock.base.Add(clock.offsets[clock.reads])
	clock.reads++
	return value
}

func (clock *scriptedClock) requireExhausted() {
	clock.t.Helper()
	if clock.reads != len(clock.offsets) {
		clock.t.Fatalf("clock reads=%d want=%d", clock.reads, len(clock.offsets))
	}
}

func millis(value int) time.Duration { return time.Duration(value) * time.Millisecond }

func durationRef(value time.Duration) *time.Duration { return &value }

func collectSpans(ctx context.Context) (context.Context, *[]SpanObservation) {
	observations := &[]SpanObservation{}
	return WithSpanObserver(ctx, func(observation SpanObservation) {
		*observations = append(*observations, observation)
	}), observations
}

func onlySpan(t *testing.T, observations []SpanObservation) SpanObservation {
	t.Helper()
	if len(observations) != 1 {
		t.Fatalf("observations=%+v want exactly one", observations)
	}
	return observations[0]
}

func applyRow(ctx context.Context, database *DB, id string) error {
	return database.Apply(ctx, []port.RecordMutation{{Bucket: "span_phase", ID: id, Data: []byte(id)}})
}

func TestSpanPhaseBoundaries(t *testing.T) {
	t.Run("success_with_commit", func(t *testing.T) {
		database := openTestDB(t)
		// started, lock acquired, callback start, commit start/end,
		// callback end, lock released, gate released.
		clock := newScriptedClock(t, 0, 7, 11, 20, 23, 31, 43, 47)
		readsAtObserver := -1
		var observations []SpanObservation
		ctx := WithSpanObserver(context.Background(), func(observation SpanObservation) {
			readsAtObserver = clock.reads
			observations = append(observations, observation)
		})
		if err := database.withSpan(ctx, clock.now, func(spanCtx context.Context) error {
			return applyRow(spanCtx, database, "one")
		}); err != nil {
			t.Fatal(err)
		}
		want := SpanObservation{
			Outcome:        SpanOutcomeSuccess,
			Acquired:       true,
			Wait:           millis(7),
			Hold:           millis(36),
			Callback:       durationRef(millis(20)),
			Commit:         durationRef(millis(3)),
			Total:          millis(47),
			CommitCount:    1,
			CommitCoverage: SpanCommitCoverageComplete,
		}
		if got := onlySpan(t, observations); !reflect.DeepEqual(got, want) {
			t.Fatalf("observation=%+v want=%+v", got, want)
		}
		clock.requireExhausted()
		if readsAtObserver != len(clock.offsets) {
			t.Fatalf("observer saw %d clock reads; total must close before the observer", readsAtObserver)
		}
	})

	t.Run("panic_after_commit", func(t *testing.T) {
		database := openTestDB(t)
		clock := newScriptedClock(t, 0, 1, 2, 3, 4, 6, 8, 9)
		ctx, observations := collectSpans(context.Background())
		sentinel := errors.New("panic sentinel")
		recovered := func() (value any) {
			defer func() { value = recover() }()
			_ = database.withSpan(ctx, clock.now, func(spanCtx context.Context) error {
				if err := applyRow(spanCtx, database, "before-panic"); err != nil {
					return err
				}
				panic(sentinel)
			})
			return nil
		}()
		if recovered != sentinel {
			t.Fatalf("recovered=%v want=%v", recovered, sentinel)
		}
		want := SpanObservation{
			Outcome:        SpanOutcomeError,
			Acquired:       true,
			Wait:           millis(1),
			Hold:           millis(7),
			Callback:       durationRef(millis(4)),
			Commit:         durationRef(millis(1)),
			Total:          millis(9),
			CommitCount:    1,
			CommitCoverage: SpanCommitCoverageComplete,
		}
		if got := onlySpan(t, *observations); !reflect.DeepEqual(got, want) {
			t.Fatalf("observation=%+v want=%+v", got, want)
		}
		clock.requireExhausted()
	})
}

func TestWithSpanReportsPanicAsErrorOutcome(t *testing.T) {
	database := openTestDB(t)
	ctx, observations := collectSpans(context.Background())
	sentinel := errors.New("panic sentinel")
	recovered := func() (value any) {
		defer func() { value = recover() }()
		_ = database.WithSpan(ctx, func(context.Context) error { panic(sentinel) })
		return nil
	}()
	if recovered != sentinel {
		t.Fatalf("recovered=%v want=%v", recovered, sentinel)
	}
	if got := onlySpan(t, *observations); got.Outcome != SpanOutcomeError {
		t.Fatalf("panic outcome=%q want=%q", got.Outcome, SpanOutcomeError)
	}
}

func TestSpanCommitCoverage(t *testing.T) {
	t.Run("no_commit_is_measured_zero", func(t *testing.T) {
		database := openTestDB(t)
		clock := newScriptedClock(t, 0, 1, 2, 3, 4, 5)
		ctx, observations := collectSpans(context.Background())
		if err := database.withSpan(ctx, clock.now, func(context.Context) error { return nil }); err != nil {
			t.Fatal(err)
		}
		got := onlySpan(t, *observations)
		if got.Commit == nil || *got.Commit != 0 || got.CommitCount != 0 ||
			got.CommitCoverage != SpanCommitCoverageComplete ||
			got.Callback == nil || *got.Callback != millis(1) {
			t.Fatalf("observation=%+v", got)
		}
		clock.requireExhausted()
	})

	t.Run("multiple_commits_are_summed_and_counted", func(t *testing.T) {
		database := openTestDB(t)
		clock := newScriptedClock(t, 0, 1, 2, 3, 5, 6, 9, 10, 12, 13)
		ctx, observations := collectSpans(context.Background())
		if err := database.withSpan(ctx, clock.now, func(spanCtx context.Context) error {
			if err := applyRow(spanCtx, database, "first"); err != nil {
				return err
			}
			return applyRow(spanCtx, database, "second")
		}); err != nil {
			t.Fatal(err)
		}
		got := onlySpan(t, *observations)
		if got.Commit == nil || *got.Commit != millis(5) || got.CommitCount != 2 ||
			got.CommitCoverage != SpanCommitCoverageComplete ||
			got.Callback == nil || *got.Callback != millis(8) || got.Total != millis(13) {
			t.Fatalf("observation=%+v", got)
		}
		clock.requireExhausted()
	})

	t.Run("failed_commit_call_is_counted", func(t *testing.T) {
		database := openTestDB(t)
		clock := newScriptedClock(t, 0, 1, 2, 3, 4, 5, 6, 7)
		parent, cancel := context.WithCancel(context.Background())
		defer cancel()
		// 세 번째 읽기는 data Commit 직전이다. 거기서 취소하면 Commit 호출은
		// 일어나지만 결정적으로 실패한다.
		clock.hooks = map[int]func(){3: cancel}
		ctx, observations := collectSpans(parent)
		err := database.withSpan(ctx, clock.now, func(spanCtx context.Context) error {
			return applyRow(spanCtx, database, "canceled-before-commit")
		})
		if err == nil {
			t.Fatal("commit after cancellation succeeded")
		}
		if _, found, getErr := database.Get("span_phase", "canceled-before-commit"); getErr != nil || found {
			t.Fatalf("failed commit left row found=%v err=%v", found, getErr)
		}
		got := onlySpan(t, *observations)
		if got.Outcome == SpanOutcomeSuccess || got.CommitCount != 1 ||
			got.Commit == nil || *got.Commit != millis(1) ||
			got.CommitCoverage != SpanCommitCoverageComplete {
			t.Fatalf("observation=%+v", got)
		}
		clock.requireExhausted()
	})

	t.Run("context_losing_put_is_unknown", func(t *testing.T) {
		database := openTestDB(t)
		clock := newScriptedClock(t, 0, 1, 2, 3, 4, 5)
		ctx, observations := collectSpans(context.Background())
		if err := database.withSpan(ctx, clock.now, func(context.Context) error {
			return database.Put("span_phase", "autocommit", []byte("v"))
		}); err != nil {
			t.Fatal(err)
		}
		got := onlySpan(t, *observations)
		if got.Commit != nil || got.CommitCoverage != SpanCommitCoverageUnknown {
			t.Fatalf("autocommit reported as measured: %+v", got)
		}
		clock.requireExhausted()
	})

	t.Run("background_apply_is_unknown", func(t *testing.T) {
		database := openTestDB(t)
		clock := newScriptedClock(t, 0, 1, 2, 3, 4, 5)
		ctx, observations := collectSpans(context.Background())
		if err := database.withSpan(ctx, clock.now, func(context.Context) error {
			return applyRow(context.Background(), database, "lost-context")
		}); err != nil {
			t.Fatal(err)
		}
		got := onlySpan(t, *observations)
		if got.Commit != nil || got.CommitCount != 0 || got.CommitCoverage != SpanCommitCoverageUnknown {
			t.Fatalf("context-losing apply reported as measured: %+v", got)
		}
		clock.requireExhausted()
	})

	t.Run("concurrent_unattributed_write_is_unknown", func(t *testing.T) {
		database := openTestDB(t)
		entered, written := make(chan struct{}), make(chan struct{})
		writerDone := make(chan error, 1)
		go func() {
			<-entered
			writerDone <- database.Put("span_phase", "outside", []byte("v"))
			close(written)
		}()
		ctx, observations := collectSpans(context.Background())
		if err := database.WithSpan(ctx, func(context.Context) error {
			close(entered)
			<-written
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if err := <-writerDone; err != nil {
			t.Fatal(err)
		}
		if got := onlySpan(t, *observations); got.Commit != nil || got.CommitCoverage != SpanCommitCoverageUnknown {
			t.Fatalf("unattributed write was not conservative: %+v", got)
		}
	})

	t.Run("unentered_callback_is_null", func(t *testing.T) {
		database := openTestDB(t)
		clock := newScriptedClock(t, 0, 4)
		canceled, cancel := context.WithCancel(context.Background())
		cancel()
		ctx, observations := collectSpans(canceled)
		err := database.withSpan(ctx, clock.now, func(context.Context) error {
			t.Fatal("callback ran after cancellation")
			return nil
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v", err)
		}
		want := SpanObservation{
			Outcome:        SpanOutcomeCanceled,
			Wait:           millis(4),
			Commit:         durationRef(0),
			Total:          millis(4),
			CommitCoverage: SpanCommitCoverageComplete,
		}
		if got := onlySpan(t, *observations); !reflect.DeepEqual(got, want) {
			t.Fatalf("observation=%+v want=%+v", got, want)
		}
		clock.requireExhausted()
	})
}

func TestSpanCancelledContenderReportsUnacquired(t *testing.T) {
	database := openTestDB(t)
	entered, release, holderDone := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		holderDone <- database.WithSpan(context.Background(), func(context.Context) error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	clock := newScriptedClock(t, 0, 5)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	ctx, observations := collectSpans(canceled)
	err := database.withSpan(ctx, clock.now, func(context.Context) error {
		t.Error("contender callback ran while the gate was held")
		return nil
	})
	close(release)
	if holderErr := <-holderDone; holderErr != nil {
		t.Fatal(holderErr)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	got := onlySpan(t, *observations)
	if got.Outcome != SpanOutcomeCanceled || got.Acquired || got.Callback != nil ||
		got.Hold != 0 || got.Wait != millis(5) || got.Total != millis(5) {
		t.Fatalf("observation=%+v", got)
	}
	clock.requireExhausted()
}

func TestSpanCommitAccumulatorIsolation(t *testing.T) {
	t.Run("nested_different_root", func(t *testing.T) {
		outer, inner := openTestDB(t), openTestDB(t)
		ctx, observations := collectSpans(context.Background())
		if err := outer.WithSpan(ctx, func(outerCtx context.Context) error {
			if err := applyRow(outerCtx, outer, "outer-first"); err != nil {
				return err
			}
			return inner.WithSpan(outerCtx, func(innerCtx context.Context) error {
				if err := applyRow(innerCtx, inner, "inner"); err != nil {
					return err
				}
				// outer root의 write는 inner context로 호출해도 outer span에 귀속된다.
				return applyRow(innerCtx, outer, "outer-through-inner")
			})
		}); err != nil {
			t.Fatal(err)
		}
		if len(*observations) != 2 {
			t.Fatalf("observations=%+v", *observations)
		}
		innerObservation, outerObservation := (*observations)[0], (*observations)[1]
		if innerObservation.CommitCount != 1 || innerObservation.CommitCoverage != SpanCommitCoverageComplete {
			t.Fatalf("inner observation=%+v", innerObservation)
		}
		if outerObservation.CommitCount != 2 || outerObservation.CommitCoverage != SpanCommitCoverageComplete {
			t.Fatalf("outer observation=%+v", outerObservation)
		}
	})

	t.Run("concurrent_roots", func(t *testing.T) {
		first, second := openTestDB(t), openTestDB(t)
		firstEntered, secondEntered := make(chan struct{}), make(chan struct{})
		type result struct {
			observations []SpanObservation
			err          error
		}
		run := func(database *DB, commits int, mine, other chan struct{}, done chan<- result) {
			ctx, observations := collectSpans(context.Background())
			err := database.WithSpan(ctx, func(spanCtx context.Context) error {
				close(mine)
				<-other
				for index := range commits {
					if err := applyRow(spanCtx, database, string(rune('a'+index))); err != nil {
						return err
					}
				}
				return nil
			})
			done <- result{observations: *observations, err: err}
		}
		firstDone, secondDone := make(chan result, 1), make(chan result, 1)
		go run(first, 1, firstEntered, secondEntered, firstDone)
		go run(second, 3, secondEntered, firstEntered, secondDone)
		for name, check := range map[string]struct {
			done    chan result
			commits int
		}{"first": {firstDone, 1}, "second": {secondDone, 3}} {
			got := <-check.done
			if got.err != nil {
				t.Fatalf("%s: %v", name, got.err)
			}
			observation := onlySpan(t, got.observations)
			if observation.CommitCount != check.commits || observation.CommitCoverage != SpanCommitCoverageComplete {
				t.Fatalf("%s observation=%+v want commits=%d", name, observation, check.commits)
			}
		}
	})
}

func TestSpanObserverReentersAfterRelease(t *testing.T) {
	database := openTestDB(t)
	reentry := errors.New("observer did not run")
	ctx := WithSpanObserver(context.Background(), func(SpanObservation) {
		reentry = database.WithSpan(context.Background(), func(innerCtx context.Context) error {
			return applyRow(innerCtx, database, "from-observer")
		})
	})
	if err := database.WithSpan(ctx, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if reentry != nil {
		t.Fatalf("observer reentry failed: %v", reentry)
	}
	if _, found, err := database.Get("span_phase", "from-observer"); err != nil || !found {
		t.Fatalf("observer write found=%v err=%v", found, err)
	}
}
