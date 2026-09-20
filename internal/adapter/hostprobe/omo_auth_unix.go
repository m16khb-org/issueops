//go:build unix

package hostprobe

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func readOmoAuthFile(path string, maxBytes int64) ([]byte, error) {
	parent := filepath.Dir(path)
	name := filepath.Base(path)
	parentFD, err := unix.Open(parent, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer unix.Close(parentFD)

	var beforePath unix.Stat_t
	if err := unix.Fstatat(parentFD, name, &beforePath, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return nil, err
	}
	if beforePath.Mode&unix.S_IFMT != unix.S_IFREG || beforePath.Mode&0o777 != 0o600 || beforePath.Size > maxBytes {
		return nil, fmt.Errorf("omo_auth_invalid")
	}
	fd, err := unix.Openat(parentFD, name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), name)
	if file == nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("omo_auth_invalid")
	}
	defer file.Close()

	beforeOpened, err := file.Stat()
	if err != nil || !beforeOpened.Mode().IsRegular() || beforeOpened.Mode().Perm() != 0o600 || beforeOpened.Size() > maxBytes {
		return nil, fmt.Errorf("omo_auth_invalid")
	}
	var openedStat unix.Stat_t
	if err := unix.Fstat(fd, &openedStat); err != nil || openedStat.Dev != beforePath.Dev || openedStat.Ino != beforePath.Ino {
		return nil, fmt.Errorf("omo_auth_invalid")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil || int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("omo_auth_invalid")
	}
	afterOpened, err := file.Stat()
	if err != nil || !stableOmoAuthInfo(beforeOpened, afterOpened) {
		return nil, fmt.Errorf("omo_auth_invalid")
	}
	var afterPath unix.Stat_t
	if err := unix.Fstatat(parentFD, name, &afterPath, unix.AT_SYMLINK_NOFOLLOW); err != nil ||
		afterPath.Dev != openedStat.Dev || afterPath.Ino != openedStat.Ino || afterPath.Mode != openedStat.Mode || afterPath.Size != openedStat.Size {
		return nil, fmt.Errorf("omo_auth_invalid")
	}
	return data, nil
}

func stableOmoAuthInfo(before, after os.FileInfo) bool {
	return os.SameFile(before, after) && before.Mode() == after.Mode() && before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
}
