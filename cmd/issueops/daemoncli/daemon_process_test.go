package daemoncli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestStartDaemonProcessReturnsStartError(t *testing.T) {
	err := startDaemonProcess(filepath.Join(t.TempDir(), "missing-issueops"), daemonPaths{Dir: t.TempDir()})

	if err == nil {
		t.Fatal("expected missing executable to fail")
	}
}

func TestStartDaemonProcessReapsChildWithoutBlockingStartup(t *testing.T) {
	for _, exitCode := range []int{0, 7} {
		t.Run(strconv.Itoa(exitCode), func(t *testing.T) {
			dir := t.TempDir()
			pidPath := filepath.Join(dir, "child.pid")
			fifo := filepath.Join(dir, "exit")
			if err := syscall.Mkfifo(fifo, 0o600); err != nil {
				t.Fatal(err)
			}
			executable := filepath.Join(dir, "daemon-fixture")
			body := "#!/bin/sh\nprintf '%s' \"$$\" > \"$ISSUEOPS_DAEMON_DIR/child.pid\"\nread code < \"$ISSUEOPS_DAEMON_DIR/exit\"\nexit \"$code\"\n"
			if err := os.WriteFile(executable, []byte(body), 0o700); err != nil {
				t.Fatal(err)
			}
			pid := 0
			t.Cleanup(func() {
				if pid == 0 {
					return
				}
				if processAlive(pid) {
					_ = syscall.Kill(pid, syscall.SIGKILL)
				}
				var status syscall.WaitStatus
				_, _ = syscall.Wait4(pid, &status, syscall.WNOHANG, nil)
			})
			started := make(chan error, 1)
			go func() { started <- startDaemonProcess(executable, daemonPaths{Dir: dir}) }()
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				data, err := os.ReadFile(pidPath)
				if err == nil {
					pid, _ = strconv.Atoi(strings.TrimSpace(string(data)))
					if pid > 0 {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
			}
			if pid == 0 {
				t.Fatal("child did not publish its PID")
			}
			select {
			case err := <-started:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("daemon startup waited for the child to exit")
			}
			// Keep both ends open until the child consumes the exit code; publishing
			// its PID does not mean the child has reached its FIFO read yet.
			fd, err := syscall.Open(fifo, syscall.O_RDWR|syscall.O_NONBLOCK, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer syscall.Close(fd)
			_, err = syscall.Write(fd, []byte(strconv.Itoa(exitCode)+"\n"))
			if err != nil {
				t.Fatal(err)
			}
			deadline = time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				if !processAlive(pid) {
					pid = 0 // Do not inspect or signal a PID after it has been reaped.
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
			t.Fatalf("exited daemon child %d remains unreaped (exit %d)", pid, exitCode)
		})
	}
}

type daemonStartFakeLock struct {
	closed bool
}

func (l *daemonStartFakeLock) Close() error {
	l.closed = true
	return nil
}
