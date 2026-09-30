package daemoncli

import (
	"context"
	"io"
	daemonapp "issueops/internal/application/daemon"
	daemoncontract "issueops/internal/contract/daemon"
	"os"
	"syscall"
	"time"
)

type daemonIdentityResponse = daemoncontract.IdentityResponse
type daemonStatusDeps struct {
	paths          func() (daemonPaths, error)
	readInstance   func(string) (daemonInstance, error)
	probeStatus    func(string) (daemonIdentityResponse, error)
	probeIdentity  func(string) (daemonInstance, error)
	processAlive   func(int) bool
	inspectProcess func(int) (daemonProcessIdentity, error)
}

type daemonProcess interface {
	Signal(os.Signal) error
	Kill() error
}

type daemonStopDeps struct {
	checkStatus    func() daemonStatus
	findProcess    func(int) (daemonProcess, error)
	inspectProcess func(int) (daemonProcessIdentity, error)
	processAlive   func(int) bool
	remove         func(string) error
	now            func() time.Time
	sleep          func(time.Duration)
}

type daemonStopCoordinatorDeps struct {
	paths       func() (daemonPaths, error)
	mkdirAll    func(string, os.FileMode) error
	acquireLock func(daemonPaths) (daemonStartLock, error)
	remove      func(string) error
	stop        func() (daemonStatus, error)
}

type daemonStartLock interface {
	Close() error
}

type daemonStartDeps struct {
	context     context.Context
	checkStatus func() daemonStatus
	paths       func() (daemonPaths, error)
	mkdirAll    func(string, os.FileMode) error
	acquireLock func(daemonPaths) (daemonStartLock, error)
	remove      func(string) error
	executable  func() (string, error)
	startDaemon func(exe string, paths daemonPaths) error
	wait        func(daemonPaths, time.Duration) (daemonStatus, error)
	waitContext func(context.Context, daemonPaths, time.Duration) (daemonStatus, error)
}

type daemonWaitDeps struct {
	now         func() time.Time
	sleep       func(time.Duration)
	checkStatus func() daemonStatus
}

func probeDaemonStatusWithDeps(deps daemonStatusDeps, socket string) (daemonIdentityResponse, error) {
	if deps.probeStatus != nil {
		return deps.probeStatus(socket)
	}
	instance, err := deps.probeIdentity(socket)
	if err != nil {
		return daemonIdentityResponse{}, err
	}
	return daemonIdentityResponse{
		OK:             true,
		Instance:       instance,
		MaxConnections: maxConnections,
		Accepting:      true,
	}, nil
}

func checkDaemonStatusWithDeps(d daemonStatusDeps) daemonStatus {
	return (daemonapp.Reader{Paths: d.paths, ReadInstance: d.readInstance, ProbeStatus: func(socket string) (daemonIdentityResponse, error) { return probeDaemonStatusWithDeps(d, socket) }, ProcessAlive: d.processAlive, InspectProcess: d.inspectProcess, IsNotExist: os.IsNotExist, MaxConnections: maxConnections, Location: time.Local}).Run()
}
func ensureDaemonRunningWithDeps(d daemonStartDeps) (daemonStatus, error) {
	return (daemonapp.Starter{Context: d.context, CheckStatus: d.checkStatus, Paths: d.paths, EnsureDir: func(dir string) error { return d.mkdirAll(dir, 0700) }, AcquireLock: func(paths daemonPaths) (io.Closer, error) { return d.acquireLock(paths) }, Remove: d.remove, Executable: d.executable, StartDaemon: d.startDaemon, Wait: func(ctx context.Context, paths daemonPaths, timeout time.Duration) (daemonStatus, error) {
		if d.waitContext != nil {
			return d.waitContext(ctx, paths, timeout)
		}
		return d.wait(paths, timeout)
	}}).Run()
}
func waitForDaemonWithDeps(paths daemonPaths, timeout time.Duration, d daemonWaitDeps) (daemonStatus, error) {
	return (daemonapp.Waiter{Now: d.now, SleepContext: func(_ context.Context, timeout time.Duration) error { d.sleep(timeout); return nil }, CheckStatus: d.checkStatus}).Run(context.Background(), paths, timeout)
}

type testDaemonProcessHandle struct{ daemonProcess }

func (p testDaemonProcessHandle) Terminate() error { return p.Signal(syscall.SIGTERM) }
func stopDaemonWithDeps(d daemonStopDeps) (daemonStatus, error) {
	return (daemonapp.Stopper{CheckStatus: d.checkStatus, FindProcess: func(pid int) (daemonapp.Process, error) {
		p, err := d.findProcess(pid)
		return testDaemonProcessHandle{p}, err
	}, InspectProcess: d.inspectProcess, ProcessAlive: d.processAlive, Remove: d.remove, Now: d.now, Sleep: d.sleep, Location: time.Local}).Run()
}
func stopDaemonCoordinatedWithDeps(d daemonStopCoordinatorDeps) (daemonStatus, error) {
	return (daemonapp.StopCoordinator{Paths: d.paths, EnsureDir: func(dir string) error { return d.mkdirAll(dir, 0700) }, AcquireLock: func(paths daemonPaths) (io.Closer, error) { return d.acquireLock(paths) }, Remove: d.remove, Stop: d.stop}).Run()
}

func waitForDaemon(paths daemonPaths, timeout time.Duration) (daemonStatus, error) {
	return daemonWaiter().Run(context.Background(), paths, timeout)
}
