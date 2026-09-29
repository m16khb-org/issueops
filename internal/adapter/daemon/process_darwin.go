//go:build darwin

package daemon

import (
	"fmt"
	contract "issueops/internal/contract/daemon"
	domain "issueops/internal/domain/daemon"
	"time"
)

func InspectProcess(pid int) (contract.ProcessIdentity, error) {
	if pid <= 0 {
		return contract.ProcessIdentity{}, fmt.Errorf("pid must be positive")
	}
	startOut, err := processFieldWithCLocale(pid, "lstart=")
	if err != nil {
		return contract.ProcessIdentity{}, fmt.Errorf("read process start time: %w", err)
	}
	exeOut, err := processFieldWithCLocale(pid, "comm=")
	if err != nil {
		return contract.ProcessIdentity{}, fmt.Errorf("read process executable: %w", err)
	}
	start, err := domain.CanonicalProcessStartTime(string(startOut), time.Local)
	if err != nil {
		return contract.ProcessIdentity{}, err
	}
	executable, err := canonicalExecutable(string(exeOut))
	if err != nil {
		return contract.ProcessIdentity{}, err
	}
	return contract.ProcessIdentity{StartTime: start, Executable: executable, ExecutablePathStable: false}, nil
}
