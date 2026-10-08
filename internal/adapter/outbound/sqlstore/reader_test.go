package sqlstore

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"issueops/internal/port"
)

func TestWalkExistingOrderedRowsAndEarlyExit(t *testing.T) {
	root := t.TempDir()
	db, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := closeRoot(root); err != nil {
			t.Error(err)
		}
	})
	for _, id := range []string{"z", "a", "m"} {
		if err := db.Put("b", id, []byte(id)); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Put("other", "foreign", []byte("foreign")); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"complete", "callback_error", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			wantErr := errors.New("stop visiting")
			ids := []string{}
			err := WalkExisting(ctx, root, "b", func(row port.RecordRow) error {
				ids = append(ids, row.ID)
				if string(row.Data) != row.ID {
					t.Fatalf("row data = %q, id = %q", row.Data, row.ID)
				}
				switch mode {
				case "callback_error":
					return wantErr
				case "cancel":
					cancel()
				}
				return nil
			})
			wantIDs := []string{"a"}
			switch mode {
			case "complete":
				wantIDs = []string{"a", "m", "z"}
				if err != nil {
					t.Fatal(err)
				}
			case "callback_error":
				if !errors.Is(err, wantErr) {
					t.Fatalf("callback error = %v", err)
				}
			case "cancel":
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancel error = %v", err)
				}
			}
			if !slices.Equal(ids, wantIDs) {
				t.Fatalf("visited = %v, want %v", ids, wantIDs)
			}
			// A completed or interrupted reader must release its read transaction.
			var busy, logFrames, checkpointed int
			if err := db.data.QueryRow("PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logFrames, &checkpointed); err != nil {
				t.Fatalf("checkpoint after reader: %v", err)
			}
			if busy != 0 || logFrames != checkpointed {
				t.Fatalf("reader retained transaction: busy=%d log=%d checkpointed=%d", busy, logFrames, checkpointed)
			}
		})
	}
}

func TestWalkExistingMissingAndCancelledDoNotCreateStore(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "cancelled"}[cancelled], func(t *testing.T) {
			parent := t.TempDir()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			wantErr := fs.ErrNotExist
			if cancelled {
				cancel()
				wantErr = context.Canceled
			}
			err := WalkExisting(ctx, filepath.Join(parent, "missing"), "b", func(port.RecordRow) error {
				t.Fatal("missing-store callback invoked")
				return nil
			})
			if !errors.Is(err, wantErr) {
				t.Fatalf("error = %v, want %v", err, wantErr)
			}
			entries, err := os.ReadDir(parent)
			if err != nil || len(entries) != 0 {
				t.Fatalf("missing read created state: %v, %v", entries, err)
			}
		})
	}
}

func TestWalkExistingReportsQueryFailure(t *testing.T) {
	root := t.TempDir()
	db, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := closeRoot(root); err != nil {
			t.Error(err)
		}
	})
	if _, err := db.data.Exec("DROP TABLE records"); err != nil {
		t.Fatal(err)
	}
	if err := WalkExisting(context.Background(), root, "b", func(port.RecordRow) error {
		t.Fatal("callback invoked after query failure")
		return nil
	}); err == nil {
		t.Fatal("missing records table accepted")
	}
}

func TestWalkExistingAfterStartsAfterResolvedCursor(t *testing.T) {
	root := t.TempDir()
	db, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := closeRoot(root); err != nil {
			t.Error(err)
		}
	})
	for _, id := range []string{"c", "a", "b"} {
		if err := db.Put("b", id, []byte(id)); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Put("other", "z", []byte("z")); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		cursor     string
		wantExists bool
		start      string
		want       []string
	}{
		{cursor: "", start: "", want: []string{"a", "b", "c"}},
		{cursor: "a", wantExists: true, start: "a", want: []string{"b", "c"}},
		{cursor: "z", start: "", want: []string{"a", "b", "c"}},
		{cursor: "bb", start: "bb", want: []string{"c"}},
	} {
		var lookedUp []bool
		ids := []string{}
		err := WalkExistingAfter(context.Background(), root, "b", tc.cursor, func(exists bool) string {
			lookedUp = append(lookedUp, exists)
			return tc.start
		}, func(row port.RecordRow) error {
			ids = append(ids, row.ID)
			return nil
		})
		if err != nil || !slices.Equal(ids, tc.want) || len(lookedUp) != 1 || lookedUp[0] != tc.wantExists {
			t.Fatalf("cursor=%q ids=%v exists=%v err=%v", tc.cursor, ids, lookedUp, err)
		}
	}
	missing := filepath.Join(t.TempDir(), "missing")
	if err := WalkExistingAfter(context.Background(), missing, "b", "a", func(bool) string { return "" }, func(port.RecordRow) error { return nil }); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing store error = %v", err)
	}
	if _, err := os.Stat(missing); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing store created: %v", err)
	}
}

func TestDeleteBeforeRemovesOnlyOlderRowsInBucket(t *testing.T) {
	db := openTestDB(t)
	for _, id := range []string{"msg-1", "msg-2", "msg-3"} {
		if err := db.Put("b", id, []byte(id)); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Put("other", "msg-1", []byte("keep")); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteBefore(context.Background(), "b", "msg-2"); err != nil {
		t.Fatal(err)
	}
	if ids, err := db.List("b"); err != nil || !slices.Equal(ids, []string{"msg-2", "msg-3"}) {
		t.Fatalf("bucket ids=%v err=%v", ids, err)
	}
	if ids, err := db.List("other"); err != nil || !slices.Equal(ids, []string{"msg-1"}) {
		t.Fatalf("other bucket ids=%v err=%v", ids, err)
	}
	if err := db.DeleteBefore(context.Background(), "b", ""); err == nil {
		t.Fatal("empty boundary accepted")
	}
}
