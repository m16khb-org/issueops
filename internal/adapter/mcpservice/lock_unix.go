//go:build darwin || linux

package mcpservice

import (
	"errors"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

var errLockHeld = errors.New("lock held")

func tryLock(f *os.File) error {
	lock := unix.Flock_t{Type: unix.F_WRLCK, Whence: io.SeekStart}
	err := unix.FcntlFlock(f.Fd(), unix.F_SETLK, &lock)
	if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EACCES) {
		return errLockHeld
	}
	return err
}

// LockHolder reports the PID holding the instance lock via F_GETLK, which
// observes without acquiring, so a status probe never races a starting server.
// A process can not observe its own fcntl lock this way.
func LockHolder(path string) (int, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer f.Close()
	lock := unix.Flock_t{Type: unix.F_WRLCK, Whence: io.SeekStart}
	if err := unix.FcntlFlock(f.Fd(), unix.F_GETLK, &lock); err != nil {
		return 0, err
	}
	if lock.Type == unix.F_UNLCK {
		return 0, nil
	}
	return int(lock.Pid), nil
}
