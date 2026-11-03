//go:build windows

package mcpservice

import (
	"errors"
	"io"
	"os"

	"golang.org/x/sys/windows"
)

var errLockHeld = errors.New("lock held")

// tryLock takes a non-blocking exclusive LockFileEx range. Like the Unix fcntl
// lock it belongs to this process and ends when the process exits.
func tryLock(f *os.File) error {
	err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, new(windows.Overlapped))
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return errLockHeld
	}
	return err
}

var errLockHolderUnobservable = errors.New("windows does not report which process holds a file lock")

// LockHolder serves the launchd and systemd supervisors, which Windows does
// not have; Service stops at requireSupervisor first. A LockFileEx range has
// no observable owner, so a present lock file is an error, not a guess.
func LockHolder(path string) (int, error) {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	return 0, errLockHolderUnobservable
}

// ServiceLogWriter returns stderr: no Windows supervisor redirects the
// server's output to the service log, so there is nothing to cap.
func ServiceLogWriter(_, stderr *os.File, _ string) io.Writer { return stderr }
