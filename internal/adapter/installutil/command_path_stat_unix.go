//go:build unix

package installutil

import (
	"os"
	"syscall"
)

func managedCommandStat(info os.FileInfo) (managedCommandStatFields, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return managedCommandStatFields{}, false
	}
	return managedCommandStatFields{
		device: uint64(stat.Dev), inode: uint64(stat.Ino), uid: uint32(stat.Uid), links: uint64(stat.Nlink),
	}, true
}

// managedCommandAdoptionSupport accepts every path: Linux renameat2 and macOS
// renameatx_np exchange two names atomically.
func managedCommandAdoptionSupport(string) error { return nil }
