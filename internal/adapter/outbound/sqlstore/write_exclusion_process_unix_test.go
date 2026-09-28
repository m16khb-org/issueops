//go:build unix

package sqlstore

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
	"issueops/internal/adapter/outbound/processlease"
)

func TestWriteExclusionSurvivesOwnerDeathUntilGitChildExits(t *testing.T) {
	root := t.TempDir()
	database, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(t.TempDir(), "input")
	if err := unix.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	fd, err := unix.Open(fifo, unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	input := os.NewFile(uintptr(fd), fifo)
	defer input.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWriteExclusionChildHelper$")
	cmd.Env = append(os.Environ(), "ISSUEOPS_WRITE_EXCLUSION_ROOT="+root, "ISSUEOPS_WRITE_EXCLUSION_FIFO="+fifo)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	var ready struct {
		PID int `json:"pid"`
	}
	if err := json.NewDecoder(bufio.NewReader(stdout)).Decode(&ready); err != nil || ready.PID <= 0 {
		t.Fatalf("child readiness=%+v err=%v", ready, err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	if err := database.Put("b", "new-owner", []byte("owner")); !errors.Is(err, processlease.ErrBusy) {
		t.Fatalf("parent death admitted writer before child exit: %v", err)
	}
	if next, err := database.ExcludeWrites(ctx); !errors.Is(err, processlease.ErrBusy) {
		if next != nil {
			next.Close()
		}
		t.Fatalf("parent death admitted competing cleanup: %v", err)
	}
	if err := input.Close(); err != nil {
		t.Fatal(err)
	}
	timer := time.NewTicker(10 * time.Millisecond)
	defer timer.Stop()
	for {
		err := database.Put("b", "new-owner", []byte("owner"))
		if err == nil {
			break
		}
		if !errors.Is(err, processlease.ErrBusy) {
			t.Fatal(err)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("child exit did not release writer exclusion: %v", ctx.Err())
		case <-timer.C:
		}
	}
	raw, found, err := database.Get("b", "new-owner")
	if err != nil || !found || string(raw) != "owner" {
		t.Fatalf("recovered write=%q found=%t err=%v", raw, found, err)
	}
}

func TestWriteExclusionChildHelper(t *testing.T) {
	root := os.Getenv("ISSUEOPS_WRITE_EXCLUSION_ROOT")
	if root == "" {
		return
	}
	database, err := Open(root)
	if err != nil {
		panic(err)
	}
	lease, err := database.ExcludeWrites(context.Background())
	if err != nil {
		panic(err)
	}
	input, err := os.Open(os.Getenv("ISSUEOPS_WRITE_EXCLUSION_FIFO"))
	if err != nil {
		panic(err)
	}
	child := exec.Command("git", "hash-object", "--stdin")
	child.Stdin = input
	processlease.Attach(lease.Context(context.Background()), child)
	if err := child.Start(); err != nil {
		panic(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]int{"pid": child.Process.Pid}); err != nil {
		panic(err)
	}
	_ = child.Wait()
	os.Exit(0)
}
