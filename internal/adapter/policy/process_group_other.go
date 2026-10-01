//go:build !unix

package policy

import "os/exec"

// Non-Unix platforms bound pipe waits but only terminate the direct process.
func configureProcessGroup(_ *exec.Cmd)         {}
func terminateProcessGroup(cmd *exec.Cmd) error { return cmd.Process.Kill() }
