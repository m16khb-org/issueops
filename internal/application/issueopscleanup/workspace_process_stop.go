package issueopscleanup

import (
	"fmt"
	"time"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

// WorkspaceProcessControl observes OS identity and sends signals. Signal errors
// are resolved by subsequent occupancy observations, preserving the stop contract.
type WorkspaceProcessControl interface {
	Observe(string) (port.CleanupWorkspaceOccupancy, error)
	RequesterPID() int
	Hangup(int)
	Terminate(int)
	Kill(int)
	Wait(time.Duration)
}

type WorkspaceProcessStopper struct{ Processes WorkspaceProcessControl }

const (
	cleanupStopGracePeriod  = 5 * time.Second
	cleanupStopKillPeriod   = 2 * time.Second
	cleanupStopPollInterval = 250 * time.Millisecond
)

func (s WorkspaceProcessStopper) Stop(root string, preview []model.CleanupWorkspaceProcess, excluded map[int]bool) ([]model.CleanupWorkspaceProcess, error) {
	occupancy, err := s.Processes.Observe(root)
	if err != nil {
		return nil, err
	}
	targets, err := domain.CleanupStopTargets(occupancy.Occupants, occupancy.Ancestry, s.Processes.RequesterPID(), preview, excluded)
	if err != nil || len(targets) == 0 {
		return nil, err
	}
	for _, target := range targets {
		s.Processes.Hangup(target.PID)
		s.Processes.Terminate(target.PID)
	}
	remaining, err := s.awaitRelease(root, targets, cleanupStopGracePeriod)
	if err != nil {
		return nil, err
	}
	if len(remaining) > 0 {
		for _, target := range remaining {
			s.Processes.Kill(target.PID)
		}
		if remaining, err = s.awaitRelease(root, remaining, cleanupStopKillPeriod); err != nil {
			return nil, err
		}
	}
	final, err := s.Processes.Observe(root)
	if err != nil {
		return nil, err
	}
	if len(remaining) > 0 || len(final.Occupants) > 0 {
		return nil, fmt.Errorf("workspace processes still occupy %s after HUP/TERM/KILL: %s", root, domain.DescribeCleanupProcesses(final.Occupants))
	}
	return targets, nil
}

// The elapsed budget advances by poll intervals so fake waits remain bounded.
func (s WorkspaceProcessStopper) awaitRelease(root string, targets []model.CleanupWorkspaceProcess, budget time.Duration) ([]model.CleanupWorkspaceProcess, error) {
	for waited := time.Duration(0); ; waited += cleanupStopPollInterval {
		occupancy, err := s.Processes.Observe(root)
		if err != nil {
			return nil, err
		}
		remaining := domain.RemainingCleanupProcesses(targets, occupancy.Occupants)
		if len(remaining) == 0 || waited >= budget {
			return remaining, nil
		}
		s.Processes.Wait(cleanupStopPollInterval)
	}
}
