// Package lintdiagnose provides the process effects for lint failure
// diagnosis: root normalization and command execution. It is an adapter-layer
// package.
package lintdiagnose

import (
	"errors"
	"os/exec"
)

// Effects runs the lint diagnosis side effects. Normalize resolves the
// repository root path.
type Effects struct{ Normalize func(string) (string, error) }

// NormalizeRoot resolves root through the injected Normalize function.
func (effects Effects) NormalizeRoot(root string) (string, error) { return effects.Normalize(root) }

// Run executes argv in root and returns its combined stdout and stderr, the
// exit code, and whether the command failed. A start failure returns -1.
func (Effects) Run(root string, argv []string) (string, int, bool) {
	command := exec.Command(argv[0], argv[1:]...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err == nil {
		return string(output), 0, false
	}
	if exitError, ok := errors.AsType[*exec.ExitError](err); ok {
		return string(output), exitError.ExitCode(), true
	}
	return string(output), -1, true
}
