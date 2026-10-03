package sqlstore

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"path/filepath"
	"testing"

	"modernc.org/sqlite"
)

func TestAcquiredSpanTransactionSurvivesRequestCancellation(t *testing.T) {
	holder := openTestDB(t)
	if err := holder.span.Close(); err != nil {
		t.Fatal(err)
	}
	recorder := &spanContextDriver{}
	sql.Register(holder.dir, recorder)
	var err error
	holder.span, err = sql.Open(holder.dir, "file:"+filepath.Join(holder.dir, spanDBFile)+"?_pragma=busy_timeout(0)&_txlock=immediate")
	if err != nil {
		t.Fatal(err)
	}
	contender, err := newDB(holder.dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		contender.data.Close()
		contender.span.Close()
	})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	tx, _, err := holder.beginSpanTx(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			t.Error(err)
		}
	}()

	cancel()
	// Observe the actual driver's transaction context, without racing the
	// database/sql goroutine that rolls back a canceled transaction.
	if err := recorder.context.Err(); err != nil {
		t.Fatalf("request cancellation reached the acquired lock: %v", err)
	}
	// The owner, not database/sql's cancellation goroutine, ends this lock.
	if _, err := tx.ExecContext(t.Context(), "SELECT 1"); err != nil {
		t.Fatalf("request cancellation ended the acquired lock: %v", err)
	}
	other, err := contender.span.BeginTx(t.Context(), nil)
	if err == nil {
		if err := other.Rollback(); err != nil {
			t.Fatal(err)
		}
		t.Fatal("contender acquired the callback's lock")
	}
	if !isSQLiteLockContention(err) {
		t.Fatalf("expected typed SQLite contention: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	other, err = contender.span.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("lock was not released by its owner: %v", err)
	}
	if err := other.Rollback(); err != nil {
		t.Fatal(err)
	}
}

type spanContextDriver struct {
	sqlite.Driver
	context context.Context
}

func (d *spanContextDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.Driver.Open(name)
	if err != nil {
		return nil, err
	}
	return &spanContextConn{Conn: conn, recorder: d}, nil
}

type spanContextConn struct {
	driver.Conn
	recorder *spanContextDriver
}

func (c *spanContextConn) BeginTx(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	c.recorder.context = ctx
	return c.Conn.(driver.ConnBeginTx).BeginTx(ctx, options)
}
