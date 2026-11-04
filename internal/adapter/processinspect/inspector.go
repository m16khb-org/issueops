// Package processinspect observes the identity (start time and executable) of
// a running process by PID, using /proc on Linux, the process handle on Windows
// and ps elsewhere. It is an adapter for the process identity contract.
package processinspect

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Inspector keeps the host observation command and environment fixed for one caller.
type Inspector struct {
	PSExecutable  string
	PSLookupError error
	Environment   []string
}

func canonicalExecutable(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("process executable is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}
