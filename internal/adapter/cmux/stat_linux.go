//go:build linux

package cmux

import (
	"fmt"
	"os"
	"syscall"
)

func fileIdentity(info os.FileInfo) (device, inode uint64, ctimeNS int64, uid, gid uint32, err error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, 0, 0, 0, fmt.Errorf("cmux endpoint stat identity is unavailable")
	}
	return uint64(stat.Dev), stat.Ino, stat.Ctim.Sec*1_000_000_000 + stat.Ctim.Nsec, stat.Uid, stat.Gid, nil
}
