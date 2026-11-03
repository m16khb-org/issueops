//go:build windows

package processinspect

import (
	"fmt"
	"time"

	"issueops/internal/adapter/outbound/osprocess"
	contract "issueops/internal/contract/processidentity"
)

// Inspect reads the creation time and image path of pid from the process
// handle; Windows has no ps, and PSExecutable is not used.
func (observer Inspector) Inspect(pid int) (contract.Identity, error) {
	if pid <= 0 {
		return contract.Identity{}, fmt.Errorf("pid must be positive")
	}
	started, image, err := osprocess.Identity(pid)
	if err != nil {
		return contract.Identity{}, fmt.Errorf("read process identity: %w", err)
	}
	executable, err := canonicalExecutable(image)
	if err != nil {
		return contract.Identity{}, err
	}
	return contract.Identity{StartTime: started.Format(time.RFC3339), Executable: executable}, nil
}
