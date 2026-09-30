package lintdiagnose

import (
	"fmt"
	"strings"
)

func ValidateCommand(argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("missing command to execute")
	}
	return nil
}
func FailureTail(output string) string {
	lines := strings.Split(output, "\n")
	if len(lines) > 150 {
		lines = lines[len(lines)-150:]
	}
	return strings.Join(lines, "\n")
}
