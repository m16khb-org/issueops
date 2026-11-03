//go:build windows

package executioncmd

import (
	"strings"

	"issueops/internal/adapter/outbound/osprocess"
)

// processCommandLine reads one process's command line from its PEB. Only an
// Omo bun candidate needs it.
func processCommandLine(pid int) string {
	line, err := osprocess.CommandLine(pid)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(line)
}
