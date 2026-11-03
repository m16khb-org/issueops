//go:build windows

package hostprobe

import (
	"errors"
	"os"
	"os/exec"

	"issueops/internal/adapter/outbound/osprocess"
)

// configureProcessGroup has nothing to set on Windows: the tree is read at
// kill time.
func configureProcessGroup(*exec.Cmd) {}

func terminateProcessTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	if err := osprocess.KillTree(cmd.Process.Pid); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

// interruptProcessTree terminates the tree: Windows has no signal a console
// child can be asked to exit with when it shares no console with the caller.
func interruptProcessTree(cmd *exec.Cmd) error { return terminateProcessTree(cmd) }
