//go:build unix

package issueopslease

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	leasedomain "issueops/internal/domain/issueopslease"
)

func observeReceipt(ctx context.Context, pid int) (leasedomain.ProcessReceipt, error) {
	if pid <= 0 {
		return leasedomain.ProcessReceipt{}, fmt.Errorf("native process pid must be positive")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	probe, cancel := context.WithTimeout(ctx, processProbeTimeout)
	defer cancel()
	// One ps call reads both fields (a spawn fewer, no PID-reuse gap between
	// them); the row splits like the whoami snapshot row.
	row, err := processField(probe, pid, "lstart=,comm=")
	if err != nil {
		return leasedomain.ProcessReceipt{}, err
	}
	fields := strings.Fields(row)
	if len(fields) < 6 {
		return leasedomain.ProcessReceipt{}, fmt.Errorf("native process executable identity is empty")
	}
	started, err := parseProcessStart(strings.Join(fields[:5], " "))
	if err != nil {
		return leasedomain.ProcessReceipt{}, err
	}
	executable := strings.Join(fields[5:], " ")
	return leasedomain.ProcessReceipt{PID: pid, StartedAt: started.UTC().Format(time.RFC3339), Executable: executable}, nil
}
func processField(ctx context.Context, pid int, field string) (string, error) {
	cmd := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-o", field)
	// lstart in UTC, parsed with time.UTC: independent of the caller's TZ.
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C", "TZ=UTC0")
	output, err := cmd.Output()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(output))
	if value == "" {
		return "", fmt.Errorf("native process field %s is empty", field)
	}
	return value, nil
}
func pidAlive(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, nil
	}
	err = process.Signal(syscall.Signal(0))
	switch {
	case err == nil, errors.Is(err, os.ErrPermission):
		return true, nil
	case errors.Is(err, os.ErrProcessDone), errors.Is(err, syscall.ESRCH):
		return false, nil
	default:
		return false, err
	}
}
