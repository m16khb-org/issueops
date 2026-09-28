//go:build unix

package processlease

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

func Acquire(ctx context.Context, directory, key string) (*Lease, error) {
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
		return nil, err
	}
	defer unix.Close(dirFD)
	var dirInfo unix.Stat_t
	if err := unix.Fstat(dirFD, &dirInfo); err != nil {
		return nil, err
	}
	if dirInfo.Mode&0777 != 0700 || dirInfo.Uid != uint32(os.Geteuid()) {
		return nil, errors.New("execution lifetime directory must be private and owned by the current user")
	}
	name := fmt.Sprintf("%x.lock", sha256.Sum256([]byte(key)))
	fd, err := unix.Openat(dirFD, name, unix.O_RDWR|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), name)
	keep := false
	defer func() {
		if !keep {
			_ = file.Close()
		}
	}()
	var info unix.Stat_t
	if err := unix.Fstat(fd, &info); err != nil {
		return nil, err
	}
	if info.Mode&unix.S_IFMT != unix.S_IFREG || info.Mode&0777 != 0600 || info.Uid != uint32(os.Geteuid()) {
		return nil, errors.New("execution lifetime lock must be a private regular file owned by the current user")
	}
	if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, ErrBusy
		}
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	keep = true
	return &Lease{file: file, directory: directory, key: key}, nil
}
