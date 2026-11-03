//go:build unix

package install

import (
	"errors"
	"os"
	"syscall"
)

// legacyPIDAlive reports whether a process with pid exists; a process owned by
// another user still counts as alive.
func legacyPIDAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}
