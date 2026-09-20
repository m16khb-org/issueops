//go:build unix

package hostprobe

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestTerminateProcessTreeSignalsTheUnixProcessGroup(t *testing.T) {
	original := killProcessGroup
	defer func() { killProcessGroup = original }()

	var gotPID int
	var gotSignal syscall.Signal
	killProcessGroup = func(pid int, signal syscall.Signal) error {
		gotPID = pid
		gotSignal = signal
		return nil
	}
	cmd := &exec.Cmd{Process: &os.Process{Pid: 43210}}
	if err := terminateProcessTree(cmd); err != nil {
		t.Fatal(err)
	}
	if gotPID != -43210 || gotSignal != syscall.SIGKILL {
		t.Fatalf("kill(%d, %v), want kill(-43210, SIGKILL)", gotPID, gotSignal)
	}
}

func TestExecRunnerTimeoutKillsAndReapsUnixDescendant(t *testing.T) {
	root := t.TempDir()
	pidPath := filepath.Join(root, "descendant.pid")
	script := "sleep 30 & child=$!; printf '%s' \"$child\" >\"$1\"; wait"
	_, err := (ExecRunner{}).Run(context.Background(), CommandRequest{
		Cwd: root, Argv: []string{"/bin/sh", "-c", script, "sh", pidPath},
		Env: []string{"PATH=/usr/bin:/bin"}, Timeout: 100 * time.Millisecond,
	})
	if err == nil || err.Error() != "command_timeout" {
		t.Fatalf("err = %v", err)
	}
	body, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(body)))
	if err != nil || pid <= 0 {
		t.Fatalf("descendant pid = %q err=%v", body, err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		err = syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("descendant %d still exists after process-group timeout: %v", pid, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestExecRunnerBoundsWaitWhenGroupTerminationDoesNotStopProcess(t *testing.T) {
	original := killProcessGroup
	defer func() { killProcessGroup = original }()
	killProcessGroup = func(int, syscall.Signal) error { return nil }

	started := time.Now()
	_, err := (ExecRunner{}).Run(context.Background(), CommandRequest{
		Argv: []string{"/bin/sleep", "3"}, Timeout: 50 * time.Millisecond,
	})
	if err == nil || err.Error() != "command_timeout" {
		t.Fatalf("err = %v", err)
	}
	if elapsed := time.Since(started); elapsed >= time.Second {
		t.Fatalf("termination wait took %s", elapsed)
	}
}
