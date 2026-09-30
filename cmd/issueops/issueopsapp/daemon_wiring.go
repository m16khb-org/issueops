package issueopsapp

import (
	"context"
	"issueops/cmd/issueops/daemoncli"
	"issueops/cmd/issueops/mcpcli"
	adapter "issueops/internal/adapter/daemon"
	app "issueops/internal/application/daemon"
	contract "issueops/internal/contract/daemon"
	domain "issueops/internal/domain/daemon"
	"net"
	"os"
	"os/exec"
	"time"
)

func newDaemonReader() app.Reader {
	paths, err := adapter.Current()
	capacity := domain.MaxConnections(os.Getenv("ISSUEOPS_DAEMON_MAX_CONNECTIONS"))
	return app.Reader{Paths: func() (contract.Paths, error) { return paths, err }, ReadInstance: adapter.ReadInstance, ProbeStatus: (adapter.Probe{MaxConnections: capacity}).Status, ProcessAlive: adapter.ProcessAlive, InspectProcess: newProcessInspector().Inspect, IsNotExist: os.IsNotExist, MaxConnections: capacity, Location: time.Local}
}
func newDaemonCommand() daemoncli.Command {
	reader := newDaemonReader()
	launcher := adapter.Launcher{Environment: os.Environ(), HarnessRoot: issueOpsRoot()}
	waiter := app.Waiter{Now: time.Now, CheckStatus: reader.Run, SleepContext: adapter.SleepContext}
	starter := app.Starter{Context: context.Background(), CheckStatus: reader.Run, Paths: reader.Paths, EnsureDir: adapter.EnsureDirectory, AcquireLock: adapter.AcquireLock, Remove: os.Remove, Executable: os.Executable, StartDaemon: launcher.Start, Wait: waiter.Run}
	stopper := app.Stopper{CheckStatus: reader.Run, FindProcess: adapter.FindProcess, InspectProcess: reader.InspectProcess, ProcessAlive: adapter.ProcessAlive, Remove: os.Remove, Now: time.Now, Sleep: time.Sleep, Location: time.Local}
	stop := app.StopCoordinator{Paths: reader.Paths, EnsureDir: adapter.EnsureDirectory, AcquireLock: adapter.AcquireLock, Remove: os.Remove, Stop: stopper.Run}
	server := newDaemonServer(reader, issueOpsMCPDependencies())
	return daemoncli.Command{Start: starter.Run, Status: reader.Run, Stop: stop.Run, Serve: server.Run}
}
func newDaemonServer(reader app.Reader, mcp mcpcli.MCPDependencies) daemoncli.Server {
	return daemoncli.Server{MaxConnections: reader.MaxConnections, IdleTimeout: domain.IdleTimeout(os.Getenv("ISSUEOPS_MCP_IDLE_TIMEOUT")), Paths: reader.Paths, MkdirAll: os.MkdirAll, OpenLog: func(path string) (daemoncli.LogFile, error) {
		return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	}, Remove: os.Remove, Listen: net.Listen, Chmod: os.Chmod, WriteInstance: adapter.WriteInstance, PID: os.Getpid, InspectProcess: reader.InspectProcess, BuildSHA: adapter.ExecutableSHA, NewToken: adapter.NewIdentityToken, Now: func() time.Time { return time.Now().UTC() }, ServeMCPStream: func(ctx context.Context, conn net.Conn, log daemoncli.LogFile) error {
		return mcpcli.ServeMCPStreamContextWithDaemonLogger(ctx, conn, conn, log, mcp)
	}}
}
func runDaemon(args []string) error { return newDaemonCommand().Run(args) }

func newProcessInspector() adapter.ProcessInspector {
	ps, err := exec.LookPath("ps")
	return adapter.ProcessInspector{PSExecutable: ps, PSLookupError: err, Environment: os.Environ()}
}
