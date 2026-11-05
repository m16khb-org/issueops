// Package lintdiagnose holds the pure rules of the lint diagnosis capability:
// command validation and failure output trimming. It performs no I/O.
package lintdiagnose

import (
	"fmt"
	"strings"
)

// ValidateCommand returns an error when argv holds no command to execute.
func ValidateCommand(argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("missing command to execute")
	}
	return nil
}

// FailureTail returns the last 150 lines of output, or all of it when shorter,
// so a diagnosis prompt stays bounded.
func FailureTail(output string) string {
	lines := strings.Split(output, "\n")
	if len(lines) > 150 {
		lines = lines[len(lines)-150:]
	}
	return strings.Join(lines, "\n")
}
