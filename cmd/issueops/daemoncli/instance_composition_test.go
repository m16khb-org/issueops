package daemoncli

import (
	"context"
	"io"
	adapter "issueops/internal/adapter/daemon"
	app "issueops/internal/application/daemon"
	contract "issueops/internal/contract/daemon"
	domain "issueops/internal/domain/daemon"
	"net"
	"os"
	"os/exec"
	"time"
)

type daemonPaths = contract.Paths
type daemonInstance = contract.InstanceRecord
type daemonProcessIdentity = contract.ProcessIdentity
type daemonStatus = contract.Status
type daemonServerLogFile = LogFile

const maxConnections = domain.DefaultMaxConnections
const defaultMaxConnections = domain.DefaultMaxConnections

const daemonProtocolVersion = contract.ProtocolVersion
const daemonIdentityRequest = contract.IdentityRequest

func daemonMaxConnections(v string) int        { return domain.MaxConnections(v) }
func currentDaemonPaths() (daemonPaths, error) { return adapter.Current() }
func processAlive(pid int) bool                { return adapter.ProcessAlive(pid) }
func probeDaemonStatus(socket string) (contract.IdentityResponse, error) {
	return (adapter.Probe{MaxConnections: maxConnections}).Status(socket)
}

func daemonExecutableSHA(p string) (string, error) { return adapter.ExecutableSHA(p) }
func checkDaemonStatus() daemonStatus {
	return (app.Reader{Paths: currentDaemonPaths, ReadInstance: adapter.ReadInstance, ProbeStatus: probeDaemonStatus, ProcessAlive: processAlive, InspectProcess: testProcessInspector().Inspect, IsNotExist: os.IsNotExist, MaxConnections: maxConnections, Location: time.Local}).Run()
}
func daemonWaiter() app.Waiter {
	return app.Waiter{Now: time.Now, CheckStatus: checkDaemonStatus, SleepContext: adapter.SleepContext}
}
func acquireDaemonLock(paths daemonPaths) (io.Closer, error) { return adapter.AcquireLock(paths) }
func startDaemonProcess(exe string, paths daemonPaths) error {
	return (adapter.Launcher{Environment: os.Environ(), HarnessRoot: "."}).Start(exe, paths)
}
func stopDaemon() (daemonStatus, error) {
	stopper := app.Stopper{CheckStatus: checkDaemonStatus, FindProcess: adapter.FindProcess, InspectProcess: testProcessInspector().Inspect, ProcessAlive: processAlive, Remove: os.Remove, Now: time.Now, Sleep: time.Sleep, Location: time.Local}
	return (app.StopCoordinator{Paths: currentDaemonPaths, EnsureDir: adapter.EnsureDirectory, AcquireLock: adapter.AcquireLock, Remove: os.Remove, Stop: stopper.Run}).Run()
}
func runDaemon(args []string) error {
	return (Command{Status: checkDaemonStatus, Stop: stopDaemon, Start: func() (daemonStatus, error) {
		return (app.Starter{Context: context.Background(), CheckStatus: checkDaemonStatus, Paths: currentDaemonPaths, EnsureDir: adapter.EnsureDirectory, AcquireLock: adapter.AcquireLock, Remove: os.Remove, Executable: os.Executable, StartDaemon: startDaemonProcess, Wait: daemonWaiter().Run}).Run()
	}, Serve: func() error { return nil }}).Run(args)
}
func serveDaemonConnection(conn net.Conn, logFile LogFile, instance daemonInstance, serve func(net.Conn, LogFile) error) error {
	return serveDaemonConnectionWithAdmission(conn, logFile, instance, newDaemonAdmission(maxConnections), func(_ context.Context, c net.Conn, l LogFile) error { return serve(c, l) })
}

func daemonStatusForMCP() daemonStatus { return checkDaemonStatus() }

func testProcessInspector() adapter.ProcessInspector {
	ps, err := exec.LookPath("ps")
	return adapter.ProcessInspector{PSExecutable: ps, PSLookupError: err, Environment: os.Environ()}
}
