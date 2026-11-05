//go:build windows

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
	"syscall"

	"golang.org/x/sys/windows"

	"issueops/internal/adapter/outbound/osfile"
)

// Windows releases a LockFileEx range when the locking process closes its
// handle, even while a child still holds an inherited copy. The open share
// mode instead lasts until every handle to the file object closes, which is
// the flock semantics the lifetime needs: an exclusive lease opens for write
// and shares nothing; a shared lease opens read-only and shares only reads.

// Acquire opens the lock file for key in directory exclusively (write access, no sharing)
// and returns the lease. It returns ErrBusy when another handle holds the file, and fails
// if ctx is done, either argument is blank, or the directory or file is not private.
func Acquire(ctx context.Context, directory, key string) (*Lease, error) {
	return acquireLease(ctx, directory, key, true)
}

// AcquireShared opens the lock file for key in directory read-only, sharing only reads,
// and returns a SharedLease that cannot provide context or drainage.
func AcquireShared(ctx context.Context, directory, key string) (*SharedLease, error) {
	lease, err := acquireLease(ctx, directory, key, false)
	if err != nil {
		return nil, err
	}
	return &SharedLease{file: lease.file}, nil
}

// inherit marks the lease handle inheritable and lists it for the child. Go
// passes children an explicit handle list, so other children never inherit it.
func inherit(cmd *exec.Cmd, file *os.File) {
	handle := windows.Handle(file.Fd())
	if err := windows.SetHandleInformation(handle, windows.HANDLE_FLAG_INHERIT, windows.HANDLE_FLAG_INHERIT); err != nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.AdditionalInheritedHandles = append(cmd.SysProcAttr.AdditionalInheritedHandles, syscall.Handle(handle))
}

func acquireLease(ctx context.Context, directory, key string, exclusive bool) (*Lease, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(directory) == "" || strings.TrimSpace(key) == "" {
		return nil, errors.New("execution lifetime directory and key are required")
	}
	if err := osfile.MkdirAll(directory, 0o700); err != nil {
		return nil, err
	}
	dirInfo, err := os.Lstat(directory)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: directory, Err: err}
	}
	if !dirInfo.IsDir() || dirInfo.Mode()&os.ModeSymlink != 0 || osfile.Perm(directory, dirInfo) != 0o700 {
		return nil, errors.New("execution lifetime directory must be private and owned by the current user")
	}
	path := filepath.Join(directory, fmt.Sprintf("%x.lock", sha256.Sum256([]byte(key))))
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	access, share := uint32(windows.GENERIC_READ), uint32(windows.FILE_SHARE_READ)
	if exclusive {
		access, share = windows.GENERIC_READ|windows.GENERIC_WRITE, 0
	}
	handle, err := windows.CreateFile(name, access, share, nil, windows.OPEN_ALWAYS,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		return nil, ErrBusy
	}
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	file := os.NewFile(uintptr(handle), path)
	keep := false
	defer func() {
		if !keep {
			_ = file.Close()
		}
	}()
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || osfile.Perm(path, info) != 0o600 {
		return nil, errors.New("execution lifetime lock must be a private regular file owned by the current user")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	keep = true
	return &Lease{file: file, directory: directory, key: key}, nil
}
