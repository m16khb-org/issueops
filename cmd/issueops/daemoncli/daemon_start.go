package daemoncli

import (
	"context"
	"io"
	"issueops/cmd/issueops/daemoncli/daemonlock"
	daemonapp "issueops/internal/application/daemon"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func ensureDaemonRunning() (daemonStatus, error) {
	return ensureDaemonRunningContext(context.Background())
}
func ensureDaemonRunningContext(ctx context.Context) (daemonStatus, error) {
	return (daemonapp.Starter{Context: ctx, CheckStatus: checkDaemonStatus, Paths: currentDaemonPaths, EnsureDir: func(dir string) error { return os.MkdirAll(dir, 0700) }, AcquireLock: func(paths daemonPaths) (io.Closer, error) { return acquireDaemonLock(paths) }, Remove: os.Remove, Executable: os.Executable, StartDaemon: startDaemonProcess, Wait: waitForDaemonContext}).Run()
}
func daemonWaiter() daemonapp.Waiter {
	return daemonapp.Waiter{Now: time.Now, CheckStatus: checkDaemonStatus, SleepContext: func(ctx context.Context, d time.Duration) error {
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-timer.C:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}}
}
func waitForDaemonContext(ctx context.Context, paths daemonPaths, timeout time.Duration) (daemonStatus, error) {
	return daemonWaiter().Run(ctx, paths, timeout)
}
func startDaemonProcess(exe string, paths daemonPaths) error {
	cmd := exec.Command(exe, "daemon", "--internal")
	cmd.Env = append(os.Environ(), "ISSUEOPS_DAEMON_DIR="+paths.Dir, "ISSUEOPS_ROOT="+IssueOpsRoot())
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	// A long-lived MCP proxy remains the daemon's parent. Reap its child on
	// exit so a zombie PID cannot block stop/start and proxy reconnection.
	go func() { _ = cmd.Wait() }()
	return nil
}
func acquireDaemonLock(paths daemonPaths) (*os.File, error) {
	return daemonlock.Acquire(paths.Lock, os.Getpid, processAlive)
}
