package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	contract "issueops/internal/contract/daemon"
)

func Current() (contract.Paths, error) {
	dir := strings.TrimSpace(os.Getenv("ISSUEOPS_DAEMON_DIR"))
	if dir == "" {
		if state := strings.TrimSpace(os.Getenv("ISSUEOPS_STATE_DIR")); state != "" {
			dir = filepath.Join(state, "daemon")
		} else if home, err := os.UserHomeDir(); err == nil && home != "" {
			dir = filepath.Join(home, ".local", "state", "issueops", "daemon")
		} else {
			return contract.Paths{}, fmt.Errorf("cannot resolve daemon directory")
		}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return contract.Paths{}, err
	}
	abs = filepath.Clean(abs)
	return contract.Paths{
		Dir:    abs,
		Socket: filepath.Join(abs, "issueops.sock"),
		PID:    filepath.Join(abs, "issueops.pid"),
		Lock:   filepath.Join(abs, "issueops.lock"),
		Log:    filepath.Join(abs, "issueops.log"),
	}, nil
}

func ReadPID(path string) int {
	record, err := ReadInstance(path)
	if err != nil {
		return 0
	}
	return record.PID
}

func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
