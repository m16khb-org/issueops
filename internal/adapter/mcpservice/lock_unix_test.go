//go:build darwin || linux

package mcpservice

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const lockHelperEnv = "ISSUEOPS_MCPSERVICE_LOCK_HELPER_STATE"

func TestLockHelperProcess(t *testing.T) {
	state := os.Getenv(lockHelperEnv)
	if state == "" {
		t.Skip("helper process only")
	}
	instance, err := AcquireInstance(state)
	if err != nil {
		os.Stdout.WriteString("ERR " + err.Error() + "\n")
		os.Exit(2)
	}
	os.Stdout.WriteString("LOCKED\n")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	_ = instance.Release()
	os.Exit(0)
}

func TestInstanceLockIsExclusiveAcrossProcessesAndObservable(t *testing.T) {
	state := t.TempDir()
	helper := exec.Command(os.Args[0], "-test.run=^TestLockHelperProcess$")
	helper.Env = append(os.Environ(), lockHelperEnv+"="+state)
	stdin, err := helper.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := helper.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	locked := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(stdout).ReadString('\n')
		locked <- strings.TrimSpace(line)
	}()
	select {
	case line := <-locked:
		if line != "LOCKED" {
			t.Fatalf("helper did not lock: %q", line)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("helper lock timeout")
	}

	if _, err := AcquireInstance(state); !errors.Is(err, ErrInstanceConflict) {
		t.Fatalf("second instance err = %v, want conflict", err)
	}
	lockPath := filepath.Join(httpDir(state), lockFileName)
	if holder, err := LockHolder(lockPath); err != nil || holder != helper.Process.Pid {
		t.Fatalf("holder = %d, %v; want %d", holder, err, helper.Process.Pid)
	}

	if _, err := stdin.Write([]byte("release\n")); err != nil {
		t.Fatal(err)
	}
	if err := helper.Wait(); err != nil {
		t.Fatal(err)
	}
	if holder, err := LockHolder(lockPath); err != nil || holder != 0 {
		t.Fatalf("holder after exit = %d, %v", holder, err)
	}
	instance, err := AcquireInstance(state)
	if err != nil {
		t.Fatalf("lock not reusable after holder exit: %v", err)
	}
	if err := instance.Release(); err != nil {
		t.Fatal(err)
	}
}
