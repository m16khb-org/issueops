//go:build windows

package osprocess

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSnapshotIdentityAndParametersDescribeTheCurrentProcess(t *testing.T) {
	entries, err := Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	var self Entry
	for _, entry := range entries {
		if entry.PID == os.Getpid() {
			self = entry
		}
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if self.PID == 0 || self.ParentPID != os.Getppid() || !strings.EqualFold(self.Executable, executable) {
		t.Fatalf("self entry = %+v, want pid %d parent %d executable %s", self, os.Getpid(), os.Getppid(), executable)
	}
	if age := time.Since(self.Started); self.Started.IsZero() || age < 0 || age > time.Hour {
		t.Fatalf("self start time = %s", self.Started)
	}
	started, image, err := Identity(os.Getpid())
	if err != nil || !started.Equal(self.Started) || image != self.Executable {
		t.Fatalf("identity = %s %s %v", started, image, err)
	}
	line, err := CommandLine(os.Getpid())
	if err != nil || !strings.Contains(line, "-test.") {
		t.Fatalf("command line = %q, %v", line, err)
	}
	cwd, err := CurrentDirectory(os.Getpid())
	wd, _ := os.Getwd()
	if err != nil || !strings.EqualFold(filepath.Clean(cwd), filepath.Clean(wd)) {
		t.Fatalf("current directory = %q, %v; want %q", cwd, err, wd)
	}
}

func TestKillTreeTerminatesDescendantsAndReportsDoneAfterwards(t *testing.T) {
	// cmd starts a child ping, so the tree has two levels.
	cmd := exec.Command("cmd.exe", "/c", "ping -n 30 127.0.0.1 >NUL")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	var child int
	for deadline := time.Now().Add(5 * time.Second); child == 0 && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		tree, err := descendants(uint32(cmd.Process.Pid))
		if err != nil {
			t.Fatal(err)
		}
		if len(tree) > 0 {
			child = int(tree[0])
		}
	}
	if child == 0 {
		t.Fatal("child process never appeared")
	}
	if err := KillTree(cmd.Process.Pid); err != nil {
		t.Fatal(err)
	}
	select {
	case <-waited:
	case <-time.After(5 * time.Second):
		t.Fatal("root did not exit")
	}
	for deadline := time.Now().Add(5 * time.Second); Alive(child); time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("descendant %d survived", child)
		}
	}
	if err := KillTree(cmd.Process.Pid); err != os.ErrProcessDone {
		t.Fatalf("second kill = %v, want os.ErrProcessDone", err)
	}
}
