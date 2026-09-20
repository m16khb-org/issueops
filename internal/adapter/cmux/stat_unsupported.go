//go:build !darwin && !linux

package cmux

import (
	"fmt"
	"os"
)

func requireSupportedPlatform() error {
	return fmt.Errorf("cmux handoff is unsupported on this platform")
}

func fileIdentity(os.FileInfo) (device, inode uint64, ctimeNS int64, uid, gid uint32, err error) {
	return 0, 0, 0, 0, 0, fmt.Errorf("cmux endpoint identity is unsupported on this platform")
}
