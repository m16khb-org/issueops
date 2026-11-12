package sqlstore

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"issueops/internal/testsupport"
)

// injectSidecarSleep replaces the retry sleep for the test. Each call is
// recorded and handed to onSleep, when set, with its 1-based index; nothing
// waits.
func injectSidecarSleep(t *testing.T, onSleep func(call int)) *[]time.Duration {
	t.Helper()
	var calls []time.Duration
	previous := sidecarRetrySleep
	sidecarRetrySleep = func(d time.Duration) {
		calls = append(calls, d)
		if onSleep != nil {
			onSleep(len(calls))
		}
	}
	t.Cleanup(func() { sidecarRetrySleep = previous })
	return &calls
}

// T5: a directory at a sidecar name is refused at once, never retried.
func TestRepairRefusesNonRegularSidecarWithoutRetry(t *testing.T) {
	root := t.TempDir()
	wal := filepath.Join(root, dataDBFile+"-wal")
	if err := os.Mkdir(wal, 0o700); err != nil {
		t.Fatal(err)
	}
	calls := injectSidecarSleep(t, nil)

	_, err := repairPrivateSQLiteFiles(root)
	if err == nil || !strings.Contains(err.Error(), "refusing non-regular SQLite file "+wal) {
		t.Fatalf("repair error=%v want refusing non-regular SQLite file %s", err, wal)
	}
	if len(*calls) != 0 {
		t.Fatalf("non-regular sidecar was retried: sleeps=%v", *calls)
	}
}

// T7: only Windows treats error 5 as a delete-pending sidecar; elsewhere it is
// EIO and a repair never sleeps.
func TestTransientSidecarErrorIsWindowsOnly(t *testing.T) {
	err := &fs.PathError{Op: "lstat", Path: dataDBFile + "-wal", Err: syscall.Errno(5)}
	if got, want := isTransientSidecarError(err), runtime.GOOS == "windows"; got != want {
		t.Fatalf("isTransientSidecarError(errno 5) on %s = %v want %v", runtime.GOOS, got, want)
	}
	if isTransientSidecarError(fs.ErrNotExist) {
		t.Fatal("a missing file is not a transient sidecar error")
	}
}

func TestRepairPermissiveSidecarNeedsNoSidecarRetry(t *testing.T) {
	root := t.TempDir()
	shm := filepath.Join(root, dataDBFile+"-shm")
	if err := os.WriteFile(shm, []byte("sidecar"), 0o600); err != nil {
		t.Fatal(err)
	}
	testsupport.MakePermissive(t, shm, 0o644)
	calls := injectSidecarSleep(t, nil)

	repaired, err := repairPrivateSQLiteFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(repaired, []string{dataDBFile + "-shm"}) {
		t.Fatalf("repaired=%v want [%s]", repaired, dataDBFile+"-shm")
	}
	assertMode(t, shm, 0o600)
	if len(*calls) != 0 {
		t.Fatalf("repair slept without a transient error: sleeps=%v", *calls)
	}
}
