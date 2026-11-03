package sqlstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/osfile"
	"issueops/internal/testsupport"
)

// keepSidecarsOpen holds one connection to the data database for the test.
// SQLite deletes the -wal and -shm sidecars when the last connection closes,
// and on Windows the pool closes every connection that returns to it, so
// without this pin the sidecars these tests exercise would not exist.
func keepSidecarsOpen(t *testing.T, d *DB) {
	t.Helper()
	if runtime.GOOS != "windows" {
		return
	}
	conn, err := d.data.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	var tables int
	if err := conn.QueryRowContext(context.Background(), "SELECT count(*) FROM sqlite_master").Scan(&tables); err != nil {
		t.Fatal(err)
	}
}

func TestMaintainTruncatesWAL(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	keepSidecarsOpen(t, d)
	// Grow the WAL well past its header with a few hundred row upserts.
	payload := strings.Repeat("x", 4096)
	for i := range 400 {
		if err := d.Put("wal", fmt.Sprintf("id-%03d", i), []byte(payload)); err != nil {
			t.Fatal(err)
		}
	}
	walPath := filepath.Join(dir, "issueops.db-wal")
	before, err := os.Stat(walPath)
	if err != nil {
		t.Fatalf("wal must exist after writes: %v", err)
	}
	if before.Size() == 0 {
		t.Fatalf("test precondition: wal must be non-empty before maintain")
	}

	result, err := d.Maintain()
	if err != nil {
		t.Fatalf("Maintain: %v", err)
	}
	if !result.Checkpointed {
		t.Fatalf("expected checkpoint to run, got %+v", result)
	}
	if result.WALBytesBefore != before.Size() {
		t.Fatalf("WALBytesBefore=%d want %d", result.WALBytesBefore, before.Size())
	}
	after, err := os.Stat(walPath)
	if err != nil {
		t.Fatalf("wal stat after maintain: %v", err)
	}
	// TRUNCATE checkpoint resets the WAL to zero bytes (or a bare header).
	if after.Size() >= before.Size() || after.Size() > 64 {
		t.Fatalf("wal not truncated: before=%d after=%d", before.Size(), after.Size())
	}
	if result.WALBytesAfter != after.Size() {
		t.Fatalf("WALBytesAfter=%d want %d", result.WALBytesAfter, after.Size())
	}

	// Data must survive the checkpoint.
	data, ok, err := d.Get("wal", "id-399")
	if err != nil || !ok || len(data) != len(payload) {
		t.Fatalf("data lost after maintain: ok=%v err=%v len=%d", ok, err, len(data))
	}
}

func TestMaintainRestoresPrivateSidecarPermissions(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	keepSidecarsOpen(t, d)
	if err := d.Put("perm", "k", []byte("v")); err != nil {
		t.Fatal(err)
	}
	wal := filepath.Join(dir, "issueops.db-wal")
	shm := filepath.Join(dir, "issueops.db-shm")
	for _, p := range []string{wal, shm} {
		testsupport.MakePermissive(t, p, 0o644)
	}

	result, err := d.Maintain()
	if err != nil {
		t.Fatalf("Maintain: %v", err)
	}
	for _, p := range []string{wal, shm} {
		info, err := os.Stat(p)
		if err != nil {
			// The wal may be gone entirely after a TRUNCATE checkpoint; absent
			// is acceptable for the permission contract.
			continue
		}
		if got := osfile.Perm(p, info); got != 0o600 {
			t.Fatalf("%s mode=%v want 0600", p, got)
		}
	}
	if len(result.PermissionsFixed) == 0 {
		t.Fatalf("expected fixed permissions reported, got %+v", result)
	}
}

// TestSidecarPermissionsUnderUmask documents how the WAL/SHM sidecars are
// created under a typical 022 umask: modernc/sqlite derives sidecar modes from
// the main database file at creation time, but real-world long-running processes have been
// observed with 0644 sidecars, so Maintain re-asserts 0600 rather than trusting
// inheritance.
func TestSidecarPermissionsUnderUmask(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Put("umask", "k", []byte("v")); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Maintain(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if got := osfile.Perm(filepath.Join(dir, e.Name()), info); got != 0o600 {
			t.Fatalf("after maintain every store file must be 0600, got %s=%v", e.Name(), got)
		}
	}
}
