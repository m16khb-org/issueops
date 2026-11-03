//go:build windows

package issueops

import (
	"errors"
	"os"
	"syscall"

	"issueops/internal/adapter/outbound/osprocess"
)

// signalCleanupProcess maps the cleanup HUP/TERM/KILL ladder to Windows.
// Windows cannot ask another console process to exit, so every signal ends
// the process; signal 0 only tests that it still runs.
func signalCleanupProcess(pid int, sig syscall.Signal) error {
	if sig == 0 {
		if osprocess.Alive(pid) {
			return nil
		}
		return syscall.ESRCH
	}
	err := osprocess.Terminate(pid)
	if errors.Is(err, os.ErrProcessDone) {
		return syscall.ESRCH
	}
	return err
}
