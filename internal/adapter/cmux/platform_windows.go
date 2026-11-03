//go:build windows

package cmux

import (
	"errors"
	"os"
	"os/exec"

	"issueops/internal/adapter/outbound/osprocess"
)

// cmux runs only on macOS and Linux and listens on a Unix socket, so Windows
// has no endpoint to observe and no prompt to deliver.
var errUnsupportedPlatform = errors.New("cmux is not supported on windows")

func fileIdentity(os.FileInfo) (device, inode uint64, ctimeNS int64, uid, gid uint32, err error) {
	return 0, 0, 0, 0, 0, errUnsupportedPlatform
}

func readPromptPlatform(string, string, string, func()) ([]byte, error) {
	return nil, errUnsupportedPlatform
}

// configureProcessGroup makes context cancellation kill the command's whole
// process tree, not only the direct child.
func configureProcessGroup(command *exec.Cmd) {
	command.Cancel = func() error {
		err := osprocess.KillTree(command.Process.Pid)
		if errors.Is(err, os.ErrProcessDone) {
			return os.ErrProcessDone
		}
		return err
	}
}
