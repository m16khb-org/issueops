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
	if !omoAuthPathStatValid(&beforePath, maxBytes) {
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
	return readOmoAuthOpened(parentFD, fd, name, file, &beforePath, maxBytes)
}

// readOmoAuthOpened reads the already-opened auth file and verifies that the
// file, its inode, and its directory entry stayed unchanged during the read.
func readOmoAuthOpened(parentFD, fd int, name string, file *os.File, beforePath *unix.Stat_t, maxBytes int64) ([]byte, error) {
	beforeOpened, err := file.Stat()
	if err != nil || !omoAuthOpenedInfoValid(beforeOpened, maxBytes) {
		return nil, fmt.Errorf("omo_auth_invalid")
	}
	var openedStat unix.Stat_t
	if err := unix.Fstat(fd, &openedStat); err != nil || !omoAuthSameInode(&openedStat, beforePath) {
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
	if err := unix.Fstatat(parentFD, name, &afterPath, unix.AT_SYMLINK_NOFOLLOW); err != nil || !omoAuthPathUnchanged(&afterPath, &openedStat) {
		return nil, fmt.Errorf("omo_auth_invalid")
	}
	return data, nil
}

func stableOmoAuthInfo(before, after os.FileInfo) bool {
	return os.SameFile(before, after) && before.Mode() == after.Mode() && before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
}

func omoAuthPathStatValid(st *unix.Stat_t, maxBytes int64) bool {
	return st.Mode&unix.S_IFMT == unix.S_IFREG && st.Mode&0o777 == 0o600 && st.Size <= maxBytes
}

func omoAuthOpenedInfoValid(info os.FileInfo, maxBytes int64) bool {
	return info.Mode().IsRegular() && info.Mode().Perm() == 0o600 && info.Size() <= maxBytes
}

func omoAuthSameInode(a, b *unix.Stat_t) bool {
	return a.Dev == b.Dev && a.Ino == b.Ino
}

func omoAuthPathUnchanged(after, opened *unix.Stat_t) bool {
	return omoAuthSameInode(after, opened) && after.Mode == opened.Mode && after.Size == opened.Size
}
