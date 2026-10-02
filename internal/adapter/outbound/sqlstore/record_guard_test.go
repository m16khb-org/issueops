package sqlstore

import (
	"context"
	"errors"
	"testing"

	"issueops/internal/port"
	authorityport "issueops/internal/port/authority"
)

var _ authorityport.RecordReader = (*DB)(nil)

type guardValueKey struct{}

// openSharedRoot는 같은 root의 uncached 핸들 두 개를 연다. 두 번째 핸들의
// span BEGIN 결과로 첫 번째 핸들의 SQLite span lock 보유 여부를 sleep 없이 판정한다.
func openSharedRoot(t *testing.T) (*DB, *DB) {
	t.Helper()
	dir := t.TempDir()
	first, err := newDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := newDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = first.data.Close()
		_ = first.span.Close()
		_ = second.data.Close()
		_ = second.span.Close()
	})
	return first, second
}

func spanLockHeldElsewhere(other *DB) (bool, error) {
	tx, err := other.span.BeginTx(context.Background(), nil)
	if err == nil {
		return false, tx.Rollback()
	}
	if isSQLiteLockContention(err) {
		return true, nil
	}
	return false, err
}

func TestWithRecordGuardBindsAfterLockAndFeedsCallback(t *testing.T) {
	database, other := openSharedRoot(t)
	nested := openTestDB(t)
	if err := database.Put("issueops_authority_v1", "grant", []byte("record")); err != nil {
		t.Fatal(err)
	}
	binds, writeChecks := 0, 0
	guarded := WithRecordGuard(context.Background(), database.dir, func(ctx context.Context, reader authorityport.RecordReader) (context.Context, error) {
		if _, spanReader := reader.(*DB); !spanReader {
			writeChecks++
			if _, found, err := reader.Get("issueops_authority_v1", "grant"); err != nil || !found {
				return nil, errors.Join(errors.New("write recheck missed the grant row"), err)
			}
			return ctx, nil
		}
		binds++
		held, err := spanLockHeldElsewhere(other)
		if err != nil || !held {
			return nil, errors.Join(errors.New("binder ran before the span lock was held"), err)
		}
		var nestedErr *NestedSpanError
		if err := database.WithSpan(ctx, func(context.Context) error { return nil }); !errors.As(err, &nestedErr) {
			return nil, errors.Join(errors.New("binder context allowed a same-root span"), err)
		}
		data, found, err := reader.Get("issueops_authority_v1", "grant")
		if err != nil || !found {
			return nil, errors.Join(errors.New("reader missed the grant row"), err)
		}
		return context.WithValue(ctx, guardValueKey{}, string(data)), nil
	})
	ctx, observations := collectSpans(guarded)
	var seen any
	err := database.WithSpan(ctx, func(spanCtx context.Context) error {
		seen = spanCtx.Value(guardValueKey{})
		if err := applyRow(spanCtx, database, "guarded"); err != nil {
			return err
		}
		return nested.WithSpan(spanCtx, func(context.Context) error { return nil })
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen != "record" {
		t.Fatalf("callback context value=%v, want binder context", seen)
	}
	if binds != 1 || writeChecks != 1 {
		t.Fatalf("span binds=%d write rechecks=%d want=1/1; other-root spans must not bind", binds, writeChecks)
	}
	if len(*observations) != 3 || (*observations)[0].Outcome != SpanOutcomeNested {
		t.Fatalf("observations=%+v want rejected same-root, nested root, outer", *observations)
	}
	if outer := (*observations)[2]; outer.CommitCount != 1 || outer.CommitCoverage != SpanCommitCoverageComplete {
		t.Fatalf("bound context lost the span accumulator: %+v", outer)
	}
}

func TestWithRecordGuardFailureStopsCallbackAndReleasesLock(t *testing.T) {
	sentinel := errors.New("authority invalid")
	cases := map[string]func(context.Context, authorityport.RecordReader) (context.Context, error){
		"bind_error": func(context.Context, authorityport.RecordReader) (context.Context, error) {
			return nil, sentinel
		},
		"nil_context": func(context.Context, authorityport.RecordReader) (context.Context, error) {
			return nil, nil
		},
		"unrelated_context": func(context.Context, authorityport.RecordReader) (context.Context, error) {
			return context.Background(), nil
		},
	}
	for name, bind := range cases {
		t.Run(name, func(t *testing.T) {
			database, other := openSharedRoot(t)
			ctx, observations := collectSpans(WithRecordGuard(context.Background(), database.dir, bind))
			called := false
			err := database.WithSpan(ctx, func(spanCtx context.Context) error {
				called = true
				return applyRow(spanCtx, database, "must-not-write")
			})
			if err == nil || called {
				t.Fatalf("guard failure err=%v callback called=%v", err, called)
			}
			if name == "bind_error" && !errors.Is(err, sentinel) {
				t.Fatalf("bind error identity lost: %v", err)
			}
			if _, found, getErr := database.Get("span_phase", "must-not-write"); getErr != nil || found {
				t.Fatalf("guard failure wrote data found=%v err=%v", found, getErr)
			}
			if held, heldErr := spanLockHeldElsewhere(other); heldErr != nil || held {
				t.Fatalf("span lock still held after guard failure held=%v err=%v", held, heldErr)
			}
			got := onlySpan(t, *observations)
			if got.Outcome != SpanOutcomeError || !got.Acquired || got.Callback != nil ||
				got.Commit == nil || *got.Commit != 0 || got.CommitCoverage != SpanCommitCoverageComplete {
				t.Fatalf("observation=%+v", got)
			}
		})
	}
}

func TestWithRecordGuardIgnoresNilArguments(t *testing.T) {
	var nilContext context.Context
	if got := WithRecordGuard(nilContext, "/state", func(ctx context.Context, _ authorityport.RecordReader) (context.Context, error) {
		return ctx, nil
	}); got != nil {
		t.Fatalf("nil context became %v", got)
	}
	base := context.Background()
	if got := WithRecordGuard(base, "/state", nil); got != base {
		t.Fatal("nil binder changed the context")
	}
}

func TestWithRecordGuardPassesOtherRootsThrough(t *testing.T) {
	guardedRoot, other := openTestDB(t), openTestDB(t)
	sentinel := errors.New("grant revoked")
	calls := 0
	ctx := WithRecordGuard(context.Background(), guardedRoot.dir, func(context.Context, authorityport.RecordReader) (context.Context, error) {
		calls++
		return nil, sentinel
	})
	if err := other.WithSpan(ctx, func(spanCtx context.Context) error {
		return applyRow(spanCtx, other, "in-span")
	}); err != nil {
		t.Fatalf("other-root span: %v", err)
	}
	if err := other.Apply(ctx, []port.RecordMutation{{Bucket: "span_phase", ID: "direct", Data: []byte(`{}`)}}); err != nil {
		t.Fatalf("other-root write: %v", err)
	}
	if calls != 0 {
		t.Fatalf("guard ran %d times for another root", calls)
	}
	if err := guardedRoot.WithSpan(ctx, func(context.Context) error { return nil }); !errors.Is(err, sentinel) || calls != 1 {
		t.Fatalf("guarded root span err=%v calls=%d", err, calls)
	}
}

func TestWithRecordGuardRechecksGuardedWritesBeforeCommit(t *testing.T) {
	database := openTestDB(t)
	grant := port.RecordMutation{Bucket: "issueops_authority_v1", ID: "caller", Data: []byte("active")}
	if err := database.Apply(context.Background(), []port.RecordMutation{grant}); err != nil {
		t.Fatal(err)
	}
	revoked := errors.New("grant revoked")
	ctx := WithRecordGuard(context.Background(), database.dir, func(ctx context.Context, reader authorityport.RecordReader) (context.Context, error) {
		if _, found, err := reader.Get(grant.Bucket, grant.ID); err != nil || !found {
			return nil, errors.Join(revoked, err)
		}
		return ctx, nil
	})
	row := func(id string) port.RecordMutation {
		return port.RecordMutation{Bucket: "span_phase", ID: id, Data: []byte(`{}`)}
	}
	if err := database.Apply(ctx, []port.RecordMutation{row("live")}); err != nil {
		t.Fatalf("write with a live grant: %v", err)
	}
	revoke := port.RecordMutation{Bucket: grant.Bucket, ID: grant.ID, Delete: true}
	if err := database.Apply(ctx, []port.RecordMutation{row("same-tx"), revoke}); !errors.Is(err, revoked) {
		t.Fatalf("write revoking its own grant err=%v, want the recheck to see the applied mutations", err)
	}
	if _, found, err := database.Get(grant.Bucket, grant.ID); err != nil || !found {
		t.Fatalf("rejected write was not rolled back: grant found=%v err=%v", found, err)
	}
	if err := database.Apply(context.Background(), []port.RecordMutation{revoke}); err != nil {
		t.Fatalf("unguarded revoke: %v", err)
	}
	live, _, err := database.Get("span_phase", "live")
	if err != nil {
		t.Fatal(err)
	}
	writes := map[string]func() error{
		"apply": func() error { return database.Apply(ctx, []port.RecordMutation{row("apply")}) },
		"compare_and_apply": func() error {
			return database.CompareAndApply(ctx, []port.ExpectedRecord{{Bucket: "span_phase", ID: "live", Data: live}}, []port.RecordMutation{row("compare_and_apply")})
		},
		"compare_and_apply_func": func() error {
			return database.CompareAndApplyFunc(ctx, nil, func() ([]port.RecordMutation, error) {
				return []port.RecordMutation{row("compare_and_apply_func")}, nil
			})
		},
		"in_span": func() error {
			return database.WithSpan(context.Background(), func(spanCtx context.Context) error {
				return database.Apply(WithRecordGuard(spanCtx, database.dir, func(context.Context, authorityport.RecordReader) (context.Context, error) { return nil, revoked }), []port.RecordMutation{row("in_span")})
			})
		},
	}
	for name, write := range writes {
		if err := write(); !errors.Is(err, revoked) {
			t.Fatalf("%s after revocation err=%v", name, err)
		}
		if _, found, err := database.Get("span_phase", name); err != nil || found {
			t.Fatalf("%s persisted after revocation found=%v err=%v", name, found, err)
		}
	}
	if err := database.Apply(context.Background(), []port.RecordMutation{row("unguarded")}); err != nil {
		t.Fatalf("unguarded write: %v", err)
	}
}
