//go:build unix

package hostprobe

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
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
