//go:build windows

package policy

import (
	"os/exec"

	"issueops/internal/adapter/outbound/osprocess"
)

// configureProcessGroup has nothing to set on Windows: the tree is read at
// kill time.
func configureProcessGroup(*exec.Cmd) {}

func terminateProcessGroup(cmd *exec.Cmd) error { return osprocess.KillTree(cmd.Process.Pid) }
