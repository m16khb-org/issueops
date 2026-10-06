//go:build !linux

package processinspect

import (
	"fmt"
	contract "issueops/internal/contract/processidentity"
	domain "issueops/internal/domain/processidentity"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func (observer Inspector) Inspect(pid int) (contract.Identity, error) {
	if pid <= 0 {
		return contract.Identity{}, fmt.Errorf("pid must be positive")
	}
	startOut, err := observer.processFieldWithCLocale(pid, "lstart=")
	if err != nil {
		return contract.Identity{}, fmt.Errorf("read process start time: %w", err)
	}
	exeOut, err := observer.processFieldWithCLocale(pid, "comm=")
	if err != nil {
		return contract.Identity{}, fmt.Errorf("read process executable: %w", err)
	}
	start, err := domain.CanonicalStartTime(string(startOut), time.Local)
	if err != nil {
		return contract.Identity{}, err
	}
	executable, err := canonicalExecutable(string(exeOut))
	if err != nil {
		return contract.Identity{}, err
	}
	return contract.Identity{StartTime: start, Executable: executable}, nil
}

// processFieldWithCLocale reads one `ps` field under the C locale so the start
// time stays parseable. Linux reads /proc instead and excludes this file.
func (observer Inspector) processFieldWithCLocale(pid int, field string) ([]byte, error) {
	if observer.PSLookupError != nil {
		return nil, observer.PSLookupError
	}
	cmd := exec.Command(observer.PSExecutable, "-p", strconv.Itoa(pid), "-o", field)
	env := make([]string, 0, len(observer.Environment)+3)
	for _, value := range observer.Environment {
		if strings.HasPrefix(value, "LANG=") ||
			strings.HasPrefix(value, "LC_ALL=") ||
			strings.HasPrefix(value, "LC_TIME=") {
			continue
		}
		env = append(env, value)
	}
	cmd.Env = append(env, "LANG=C", "LC_ALL=C", "LC_TIME=C")
	return cmd.Output()
}
