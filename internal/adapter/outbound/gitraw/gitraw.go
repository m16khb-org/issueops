// Package gitraw runs one git command and reports its exit code and output
// byte-for-byte.
//
// Machine-delimited git output (-z path lists, binary diffs, status) must reach
// a parser unmodified, so Run neither trims nor decodes it. The package holds no
// policy and no state: callers decide which command to run, what a non-zero
// exit means, and whether output is trimmed for display.
package gitraw

import (
	"bytes"
	"errors"
	"os/exec"

	"issueops/internal/adapter/outbound/gitbin"
)

// Run runs git with args in dir. A command that ran and exited non-zero reports
// that exit code with its stdout and stderr. A command that could not run
// reports code 1 with the launch error in stderr.
func Run(dir string, args ...string) (code int, stdout, stderr string) {
	cmd := exec.Command(gitbin.Path(), args...)
	cmd.Dir = dir
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	if err == nil {
		return 0, out.String(), errOut.String()
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode(), out.String(), errOut.String()
	}
	return 1, out.String(), err.Error()
}
