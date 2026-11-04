//go:build unix

package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Install runs scripts/install-native.sh under root with args, forwarding the runtime's
// environment and standard streams. It fails if the script is missing or exits non-zero.
func (r Runtime) Install(root string, args []string) error {
	script := filepath.Join(root, "scripts", "install-native.sh")
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("install script not found at %s: %w", script, err)
	}
	command := exec.Command(script, args...)
	command.Dir = root
	command.Env = r.Environment
	command.Stdin = r.Input
	command.Stdout = r.Output
	command.Stderr = r.Diagnostics
	return command.Run()
}

// processRows returns `ps -axo pid=,ppid=,command=` rows.
func (r Runtime) processRows() ([]byte, error) {
	return r.ProcessTable.Read("-axo", "pid=,ppid=,command=")
}
