//go:build unix

package osfile

import (
	"io/fs"
	"os"
	"syscall"
)

// ExecutableName returns the file name of an executable called base.
func ExecutableName(base string) string { return base }

// Replace renames oldpath onto newpath, replacing it atomically.
func Replace(oldpath, newpath string) error { return os.Rename(oldpath, newpath) }

// WriteFile writes data to name like os.WriteFile.
func WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(name, data, perm)
}

// Perm returns the permission bits of info, which describes path.
func Perm(_ string, info fs.FileInfo) fs.FileMode { return info.Mode().Perm() }

// Chmod sets the permission bits of path.
func Chmod(path string, mode fs.FileMode) error { return os.Chmod(path, mode) }

// MkdirAll creates path and missing parents with perm.
func MkdirAll(path string, perm fs.FileMode) error { return os.MkdirAll(path, perm) }

// Mkdir creates path with perm.
func Mkdir(path string, perm fs.FileMode) error { return os.Mkdir(path, perm) }

// OpenFile opens name like os.OpenFile.
func OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}

// FileChmod sets the permission bits of the open file f.
func FileChmod(f *os.File, mode fs.FileMode) error { return f.Chmod(mode) }

// PathIdentity returns the identity of the entry info describes. path names
// the same entry; Unix reads everything from info.
func PathIdentity(_ string, info fs.FileInfo) (Identity, bool) { return statIdentity(info) }

// FileIdentity returns the identity of the open file info was read from.
func FileIdentity(_ *os.File, info fs.FileInfo) (Identity, bool) { return statIdentity(info) }

func statIdentity(info fs.FileInfo) (Identity, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return Identity{}, false
	}
	return Identity{Device: uint64(stat.Dev), Inode: uint64(stat.Ino), Links: uint64(stat.Nlink)}, true
}
