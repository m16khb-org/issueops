package daemon

import (
	"fmt"
	"io"
	daemoncontract "issueops/internal/contract/daemon"
	daemondomain "issueops/internal/domain/daemon"
	"time"
)

type Process interface {
	Terminate() error
	Kill() error
}
type Stopper struct {
	CheckStatus    func() daemoncontract.Status
	FindProcess    func(int) (Process, error)
	InspectProcess func(int) (daemoncontract.ProcessIdentity, error)
	ProcessAlive   func(int) bool
	Remove         func(string) error
	Now            func() time.Time
	Sleep          func(time.Duration)
	Location       *time.Location
}
type StopCoordinator struct {
	Paths       func() (daemoncontract.Paths, error)
	EnsureDir   func(string) error
	AcquireLock func(daemoncontract.Paths) (io.Closer, error)
	Remove      func(string) error
	Stop        func() (daemoncontract.Status, error)
}

func (deps StopCoordinator) Run() (daemoncontract.Status, error) {
	paths, err := deps.Paths()
	if err != nil {
		return daemoncontract.Status{}, err
	}
	if err := deps.EnsureDir(paths.Dir); err != nil {
		return daemoncontract.Status{}, err
	}
	lock, err := deps.AcquireLock(paths)
	if err != nil {
		return daemoncontract.Status{OK: false, Paths: paths, Message: err.Error()}, err
	}
	defer func() {
		_ = lock.Close()
		_ = deps.Remove(paths.Lock)
	}()
	return deps.Stop()
}
func (deps Stopper) Run() (daemoncontract.Status, error) {
	status := deps.CheckStatus()
	if status.Code == daemoncontract.StatusStopped && status.PID == 0 && !status.Reachable {
		status.OK = true
		status.Running = false
		status.Message = "issueops daemon already stopped"
		return status, nil
	}
	// Stale artifacts: the pid file references a dead process and the socket
	// is absent. Stopping is idempotent here — clean the stale files instead
	// of refusing, so post-install refresh (stop → start) cannot wedge on a
	// leftover pid file from a crashed or externally killed daemon.
	if status.Code == daemoncontract.StatusStopped && !status.Running && !status.Reachable && status.PID > 0 && !deps.ProcessAlive(status.PID) {
		return stoppedStatus(status, status.PID, deps.Remove, "issueops daemon already stopped (cleaned stale pid file)"), nil
	}
	if !daemondomain.CanStop(status) {
		status.OK = false
		return status, fmt.Errorf("refusing to stop unverified daemon: %s", status.Code)
	}
	instance := *status.Instance
	proc, err := deps.FindProcess(instance.PID)
	if err != nil {
		status.OK = false
		status.Message = err.Error()
		return status, err
	}
	processIdentity, err := deps.InspectProcess(instance.PID)
	if err != nil || !daemondomain.ProcessIdentityMatches(instance, processIdentity, deps.Location) {
		status = daemondomain.IdentityMismatchStatus(status, "daemon OS process identity changed before stop")
		return status, fmt.Errorf("refusing to signal unverified daemon process")
	}
	if err := proc.Terminate(); err != nil {
		status.OK = false
		status.Message = err.Error()
		return status, err
	}
	deadline := deps.Now().Add(3 * time.Second)
	for deps.Now().Before(deadline) {
		if !deps.ProcessAlive(instance.PID) {
			return stoppedStatus(status, instance.PID, deps.Remove, "issueops daemon stopped"), nil
		}
		deps.Sleep(50 * time.Millisecond)
	}
	if !deps.ProcessAlive(instance.PID) {
		return stoppedStatus(status, instance.PID, deps.Remove, "issueops daemon stopped"), nil
	}
	processIdentity, err = deps.InspectProcess(instance.PID)
	if err != nil || !daemondomain.ProcessIdentityMatches(instance, processIdentity, deps.Location) {
		// TERM 처리 직후 종료된 프로세스를 PID 재사용으로 오인하지 않도록
		// 강제 종료 직전의 생존 상태를 다시 확인한다.
		if !deps.ProcessAlive(instance.PID) {
			return stoppedStatus(status, instance.PID, deps.Remove, "issueops daemon stopped"), nil
		}
		status = daemondomain.IdentityMismatchStatus(status, "daemon OS process identity changed before forced stop")
		return status, fmt.Errorf("refusing to kill unverified daemon process")
	}
	if err := proc.Kill(); err != nil {
		status.OK = false
		status.Message = err.Error()
		return status, err
	}
	return stoppedStatus(status, instance.PID, deps.Remove, "issueops daemon killed after timeout"), nil
}
func stoppedStatus(status daemoncontract.Status, pid int, remove func(string) error, message string) daemoncontract.Status {
	_ = remove(status.Paths.Socket)
	_ = remove(status.Paths.PID)
	status.OK = true
	status.Running = false
	status.Reachable = false
	status.IdentityVerified = false
	status.PID = pid
	status.Code = daemoncontract.StatusStopped
	status.Message = message
	return status
}
