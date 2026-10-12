package sqlstore

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"issueops/internal/port"
)

func TestIsSQLiteLockContentionRecognizesTableLockedAsContention(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "locked.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(t.Context(), `CREATE TABLE t (id INTEGER)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(t.Context(), `INSERT INTO t VALUES (1)`); err != nil {
		t.Fatal(err)
	}
	var dropErr error
	err = conn.Raw(func(raw any) error {
		queryer := raw.(driver.QueryerContext)
		rows, err := queryer.QueryContext(t.Context(), `SELECT id FROM t`, nil)
		if err != nil {
			return err
		}
		defer rows.Close()
		_, dropErr = raw.(driver.ExecerContext).ExecContext(t.Context(), `DROP TABLE t`, nil)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if dropErr == nil {
		t.Fatal("dropping a table with an active reader must fail with SQLITE_LOCKED")
	}
	if !isSQLiteLockContention(dropErr) {
		t.Fatalf("SQLITE_LOCKED must count as lock contention: %v", dropErr)
	}
	if isSQLiteLockContention(os.ErrNotExist) {
		t.Fatal("non-SQLite error must not count as lock contention")
	}
}

func TestVerifyExpectedRecordsRejectsLaterMismatchAfterEarlierMatch(t *testing.T) {
	db := openTestDB(t)
	if err := db.Apply(t.Context(), []port.RecordMutation{
		{Bucket: "resume", ID: "first", Data: []byte("one")},
		{Bucket: "resume", ID: "second", Data: []byte("two")},
	}); err != nil {
		t.Fatal(err)
	}
	err := db.CompareAndApply(t.Context(), []port.ExpectedRecord{
		{Bucket: "resume", ID: "first", Data: []byte("one")},
		{Bucket: "resume", ID: "second", Data: []byte("stale")},
	}, []port.RecordMutation{{Bucket: "resume", ID: "first", Delete: true}})
	if stale, ok := errors.AsType[*RawCASError](err); !ok || stale.ID != "second" {
		t.Fatalf("later mismatch must fail with RawCASError for second: %v", err)
	}
	if _, ok, err := db.Get("resume", "first"); err != nil || !ok {
		t.Fatalf("rejected compare-and-apply must not mutate: ok=%t err=%v", ok, err)
	}
}

func TestWithSpanReportsLockFailureWithoutCancelledContext(t *testing.T) {
	db, err := newDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.span.Close(); err != nil {
		t.Fatal(err)
	}
	called := false
	err = db.WithSpan(t.Context(), func(context.Context) error {
		called = true
		return nil
	})
	if called {
		t.Fatal("callback ran without the span lock")
	}
	if err == nil || !strings.HasPrefix(err.Error(), "sqlstore span lock "+db.dir+": ") || strings.Contains(err.Error(), "%!w") || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("span lock failure must wrap the begin error: %v", err)
	}
}

func TestOpenRetriesWhileSpanDatabaseIsLockedByAnotherWriter(t *testing.T) {
	dir := t.TempDir()
	spanPath := filepath.Join(dir, spanDBFile)
	if err := os.WriteFile(spanPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(spanPath, 0o600); err != nil {
		t.Fatal(err)
	}
	holder, err := sql.Open("sqlite", "file:"+spanPath+"?_pragma=busy_timeout(0)")
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	conn, err := holder.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(t.Context(), `BEGIN IMMEDIATE`); err != nil {
		t.Fatal(err)
	}
	release := time.AfterFunc(100*time.Millisecond, func() {
		_, _ = conn.ExecContext(context.Background(), `ROLLBACK`)
	})
	defer release.Stop()
	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open must retry SQLite lock contention until the writer releases: %v", err)
	}
	if db == nil {
		t.Fatal("Open returned no handle")
	}
}
