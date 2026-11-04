//go:build unix

package processlease

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// Acquire takes the exclusive non-blocking flock lease for key in directory. It returns
// ErrBusy when another holder owns the lock, and fails if ctx is already done, either
// argument is blank, or the lock directory or file is not private to the current user.
func Acquire(ctx context.Context, directory, key string) (*Lease, error) {
	return acquireLease(ctx, directory, key, unix.LOCK_EX)
}

// AcquireShared takes a shared non-blocking flock lease for key in directory. Unlike
// Acquire it returns a SharedLease that cannot provide context or drainage.
func AcquireShared(ctx context.Context, directory, key string) (*SharedLease, error) {
	lease, err := acquireLease(ctx, directory, key, unix.LOCK_SH)
	if err != nil {
		return nil, err
	}
	return &SharedLease{file: lease.file}, nil
}

func inherit(cmd *exec.Cmd, file *os.File) { cmd.ExtraFiles = append(cmd.ExtraFiles, file) }

func acquireLease(ctx context.Context, directory, key string, mode int) (*Lease, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(directory) == "" || strings.TrimSpace(key) == "" {
		return nil, errors.New("execution lifetime directory and key are required")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	dirFD, err := unix.Open(directory, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		// Probe: a read-only state directory failed a mutation with only
		// "permission denied"; the path says which directory to fix.
		return nil, &os.PathError{Op: "open", Path: directory, Err: err}
	}
	defer unix.Close(dirFD)
	if err := validateLockDirectory(dirFD); err != nil {
		return nil, err
	}
	name := fmt.Sprintf("%x.lock", sha256.Sum256([]byte(key)))
	fd, err := unix.Openat(dirFD, name, unix.O_RDWR|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: filepath.Join(directory, name), Err: err}
	}
	file := os.NewFile(uintptr(fd), name)
	keep := false
	defer func() {
		if !keep {
			_ = file.Close()
		}
	}()
	if err := validateLockFile(fd); err != nil {
		return nil, err
	}
	if err := flockNonBlocking(fd, mode); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	keep = true
	return &Lease{file: file, directory: directory, key: key}, nil
}

func validateLockDirectory(dirFD int) error {
	var dirInfo unix.Stat_t
	if err := unix.Fstat(dirFD, &dirInfo); err != nil {
		return err
	}
	if dirInfo.Mode&0777 != 0700 || dirInfo.Uid != uint32(os.Geteuid()) {
		return errors.New("execution lifetime directory must be private and owned by the current user")
	}
	return nil
}

func validateLockFile(fd int) error {
	var info unix.Stat_t
	if err := unix.Fstat(fd, &info); err != nil {
		return err
	}
	if info.Mode&unix.S_IFMT != unix.S_IFREG || info.Mode&0777 != 0600 || info.Uid != uint32(os.Geteuid()) {
		return errors.New("execution lifetime lock must be a private regular file owned by the current user")
	}
	return nil
}

// flockNonBlocking은 비차단 flock을 시도하고 경합은 ErrBusy로 변환한다.
func flockNonBlocking(fd, mode int) error {
	if err := unix.Flock(fd, mode|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return ErrBusy
		}
		return err
	}
	return nil
}
