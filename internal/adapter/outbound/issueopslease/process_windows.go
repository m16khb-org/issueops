//go:build windows

package issueopslease

import (
	"context"
	"fmt"
	"time"

	"issueops/internal/adapter/outbound/osprocess"
	leasedomain "issueops/internal/domain/issueopslease"
)

// observeReceipt reads the creation time, truncated to seconds like ps lstart,
// and the image path of pid.
func observeReceipt(ctx context.Context, pid int) (leasedomain.ProcessReceipt, error) {
	if pid <= 0 {
		return leasedomain.ProcessReceipt{}, fmt.Errorf("native process pid must be positive")
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return leasedomain.ProcessReceipt{}, err
		}
	}
	started, executable, err := osprocess.Identity(pid)
	if err != nil {
		return leasedomain.ProcessReceipt{}, err
	}
	return leasedomain.ProcessReceipt{PID: pid, StartedAt: started.Format(time.RFC3339), Executable: executable}, nil
}

func pidAlive(pid int) (bool, error) { return osprocess.Alive(pid), nil }
