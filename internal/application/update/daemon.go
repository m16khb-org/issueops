package update

import (
	contract "issueops/internal/contract/update"
	domain "issueops/internal/domain/install"
)

type StaleDaemons struct {
	List       func() ([]contract.DaemonProcess, error)
	Terminate  func(int) error
	CurrentPID func() int
}

func (s StaleDaemons) Run() (int, error) {
	processes, err := s.List()
	if err != nil {
		return 0, err
	}
	current := s.CurrentPID()
	terminated := 0
	for _, process := range processes {
		if !domain.MayTerminateDaemon(process.PID, current) {
			continue
		}
		if err := s.Terminate(process.PID); err != nil {
			return terminated, err
		}
		terminated++
	}
	return terminated, nil
}

type DaemonRefresh struct {
	Stop    func() error
	Cleanup StaleDaemons
}

func (s DaemonRefresh) Run() error {
	if err := s.Stop(); err != nil {
		return err
	}
	_, err := s.Cleanup.Run()
	return err
}
