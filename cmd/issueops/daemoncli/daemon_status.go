package daemoncli

import (
	"io"
	"issueops/cmd/issueops/daemoncli/daemonpaths"
	daemonapp "issueops/internal/application/daemon"
	"os"
	"syscall"
	"time"
)

func checkDaemonStatus() daemonStatus {
	return (daemonapp.Reader{Paths: currentDaemonPaths, ReadInstance: daemonpaths.ReadInstance, ProbeStatus: probeDaemonStatus, ProcessAlive: processAlive, InspectProcess: daemonpaths.InspectProcess, IsNotExist: os.IsNotExist, MaxConnections: maxConnections, Location: time.Local}).Run()
}

type daemonProcessHandle struct{ *os.Process }

func (p daemonProcessHandle) Terminate() error { return p.Signal(syscall.SIGTERM) }
func stopDaemon() (daemonStatus, error) {
	stopper := daemonapp.Stopper{CheckStatus: checkDaemonStatus, FindProcess: func(pid int) (daemonapp.Process, error) {
		p, err := os.FindProcess(pid)
		return daemonProcessHandle{p}, err
	}, InspectProcess: daemonpaths.InspectProcess, ProcessAlive: processAlive, Remove: os.Remove, Now: time.Now, Sleep: time.Sleep, Location: time.Local}
	return (daemonapp.StopCoordinator{Paths: currentDaemonPaths, EnsureDir: func(dir string) error { return os.MkdirAll(dir, 0700) }, AcquireLock: func(paths daemonPaths) (io.Closer, error) { return acquireDaemonLock(paths) }, Remove: os.Remove, Stop: stopper.Run}).Run()
}
func daemonStatusForMCP() daemonStatus { return checkDaemonStatus() }
