//go:build windows

package sqlstore

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"issueops/internal/adapter/outbound/osfile"
	"issueops/internal/testsupport"
)

// makeDeletePending marks path for classic (non-POSIX) deletion through a
// handle it keeps open, the state SQLite leaves a -wal or -shm in when it
// deletes the file while another process still holds it. The name stays
// visible and every open or attribute query answers ERROR_ACCESS_DENIED until
// the returned close function runs; then the file is gone. os.Remove would use
// POSIX deletion on Windows 11 and remove the name at once.
func makeDeletePending(t *testing.T, path string) (closeHandle func()) {
	t.Helper()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(name, windows.DELETE|windows.SYNCHRONIZE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	del := byte(1)
	if err := windows.SetFileInformationByHandle(h, windows.FileDispositionInfo, &del, 1); err != nil {
		_ = windows.CloseHandle(h)
		t.Fatal(err)
	}
	closed := false
	closeHandle = func() {
		if closed {
			return
		}
		closed = true
		if err := windows.CloseHandle(h); err != nil {
			t.Errorf("close delete-pending handle: %v", err)
		}
	}
	t.Cleanup(closeHandle)
	if _, err := os.Lstat(path); !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		t.Fatalf("fixture %s is not delete-pending: lstat err=%v", path, err)
	}
	return closeHandle
}

func privateTestRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "state")
	if err := ensurePrivateRoot(root); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeDeletePendingFile(t *testing.T, path string) (closeHandle func()) {
	t.Helper()
	if err := os.WriteFile(path, []byte("sidecar"), 0o600); err != nil {
		t.Fatal(err)
	}
	return makeDeletePending(t, path)
}

func totalSleep(calls []time.Duration) time.Duration {
	var total time.Duration
	for _, d := range calls {
		total += d
	}
	return total
}

// T1: a sidecar SQLite is deleting while another process holds it is retried
// until the deletion completes.
func TestRepairRetriesDeletePendingSidecarUntilItIsGone(t *testing.T) {
	root := privateTestRoot(t)
	wal := filepath.Join(root, dataDBFile+"-wal")
	release := writeDeletePendingFile(t, wal)
	calls := injectSidecarSleep(t, func(call int) {
		if call == 1 {
			release()
		}
	})

	repaired, err := repairPrivateSQLiteFiles(root)
	if err != nil {
		t.Fatalf("repair failed on a delete-pending sidecar: %v", err)
	}
	if len(repaired) != 0 {
		t.Fatalf("repaired=%v want none", repaired)
	}
	if _, err := os.Lstat(wal); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("delete-pending sidecar still present after release: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("sleeps=%v want exactly one", *calls)
	}
}

// T2: a sidecar recreated with a permissive ACL after the deletion completes
// is repaired to owner-only and reported.
func TestRepairRetriedSidecarRecreatedPermissiveIsRepaired(t *testing.T) {
	root := privateTestRoot(t)
	wal := filepath.Join(root, dataDBFile+"-wal")
	release := writeDeletePendingFile(t, wal)
	calls := injectSidecarSleep(t, func(call int) {
		if call != 1 {
			return
		}
		release()
		if err := os.WriteFile(wal, []byte("recreated"), 0o600); err != nil {
			t.Fatal(err)
		}
		testsupport.MakePermissive(t, wal, 0o644)
		info, err := os.Lstat(wal)
		if err != nil {
			t.Fatal(err)
		}
		if got := osfile.Perm(wal, info); got == 0o600 {
			t.Fatalf("recreated sidecar fixture is already owner-only")
		}
	})

	repaired, err := repairPrivateSQLiteFiles(root)
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	assertMode(t, wal, 0o600)
	if !slices.Contains(repaired, dataDBFile+"-wal") {
		t.Fatalf("repaired=%v lacks %s", repaired, dataDBFile+"-wal")
	}
	if len(*calls) != 1 {
		t.Fatalf("sleeps=%v want exactly one", *calls)
	}
}

// T3: one sleep budget covers every sidecar of a repair call and is spent
// exactly. The -wal is released on the third sleep; the -shm stays
// access-denied, so the call fails closed with the -shm error once the sleeps
// the two files requested together reach sidecarRetryBound.
func TestRepairDeletePendingSidecarFailsClosedAfterBound(t *testing.T) {
	root := privateTestRoot(t)
	wal := filepath.Join(root, dataDBFile+"-wal")
	shm := filepath.Join(root, dataDBFile+"-shm")
	releaseWAL := writeDeletePendingFile(t, wal)
	writeDeletePendingFile(t, shm)
	calls := injectSidecarSleep(t, func(call int) {
		if call == 3 {
			releaseWAL()
		}
	})

	_, err := repairPrivateSQLiteFiles(root)
	assertSidecarBudgetSpent(t, err, shm, *calls)
	if _, err := os.Lstat(wal); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("released -wal still present: %v", err)
	}
}

// T3 through Open: a repair that spends the sleep budget yields no handle and
// caches none for the root.
func TestOpenDeletePendingSidecarCachesNoHandle(t *testing.T) {
	root := privateTestRoot(t)
	shm := filepath.Join(root, dataDBFile+"-shm")
	writeDeletePendingFile(t, shm)
	calls := injectSidecarSleep(t, nil)

	d, err := Open(root)
	if d != nil {
		t.Fatalf("Open returned a handle for a root whose sidecar stays access-denied")
	}
	assertSidecarBudgetSpent(t, err, shm, *calls)
	handlesMu.Lock()
	_, cached := handles[root]
	handlesMu.Unlock()
	if cached {
		t.Fatalf("Open cached a handle for %s after the repair failed", root)
	}
}

func assertSidecarBudgetSpent(t *testing.T, err error, path string, calls []time.Duration) {
	t.Helper()
	if err == nil {
		t.Fatal("repair accepted a sidecar that stayed access-denied")
	}
	if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		t.Fatalf("error lost ERROR_ACCESS_DENIED: %v", err)
	}
	if !strings.Contains(err.Error(), "sqlstore inspect permissions "+path) {
		t.Fatalf("error does not name %s with the inspect-permissions text: %v", path, err)
	}
	if len(calls) < 2 {
		t.Fatalf("sleeps=%v want at least two", calls)
	}
	if total := totalSleep(calls); total != sidecarRetryBound {
		t.Fatalf("total requested sleep %v (sleeps=%v) want exactly the bound %v", total, calls, sidecarRetryBound)
	}
}

// T4: the main database file is never retried; access-denied there is real.
func TestRepairDeletePendingMainDatabaseFailsWithoutSidecarRetry(t *testing.T) {
	root := privateTestRoot(t)
	dataPath := filepath.Join(root, dataDBFile)
	writeDeletePendingFile(t, dataPath)
	calls := injectSidecarSleep(t, nil)

	_, err := repairPrivateSQLiteFiles(root)
	if err == nil || !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		t.Fatalf("repair error=%v want ERROR_ACCESS_DENIED", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("main database was retried: sleeps=%v", *calls)
	}
}
