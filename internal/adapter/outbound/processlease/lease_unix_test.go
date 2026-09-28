//go:build unix

package processlease

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestLeaseExcludesSameCycleAndPreservesStableFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "leases")
	lease, err := Acquire(context.Background(), dir, "cycle")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
	before, err := os.Stat(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if next, err := Acquire(context.Background(), dir, "cycle"); !errors.Is(err, ErrBusy) {
		if next != nil {
			next.Close()
		}
		t.Fatalf("same cycle admitted: %v", err)
	}
	other, err := Acquire(context.Background(), dir, "other")
	if err != nil {
		t.Fatal(err)
	}
	other.Close()
	drained, err := lease.Drain(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer drained.Close()
	after, err := os.Stat(filepath.Join(dir, entries[0].Name()))
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("lock inode changed: %v", err)
	}
	if err := drained.Close(); err != nil {
		t.Fatal(err)
	}
	next, err := Acquire(context.Background(), dir, "cycle")
	if err != nil {
		t.Fatal(err)
	}
	next.Close()
	if _, err := os.Stat(filepath.Join(dir, entries[0].Name())); err != nil {
		t.Fatalf("lock file removed: %v", err)
	}
}

func TestLeaseRefusesCancelledAndUnsafePaths(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if lease, err := Acquire(ctx, filepath.Join(t.TempDir(), "leases"), "cycle"); !errors.Is(err, context.Canceled) {
		if lease != nil {
			lease.Close()
		}
		t.Fatalf("cancel ignored: %v", err)
	}
	for _, name := range []string{"directory symlink", "file symlink", "public directory", "public file"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "leases")
			lease, err := Acquire(context.Background(), dir, "cycle")
			if err != nil {
				t.Fatal(err)
			}
			lease.Close()
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, entries[0].Name())
			switch name {
			case "directory symlink":
				renamed := dir + "-actual"
				if err := os.Rename(dir, renamed); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(renamed, dir); err != nil {
					t.Fatal(err)
				}
			case "file symlink":
				target := filepath.Join(root, "target")
				if err := os.Rename(path, target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			case "public directory":
				if err := os.Chmod(dir, 0755); err != nil {
					t.Fatal(err)
				}
			case "public file":
				if err := os.Chmod(path, 0644); err != nil {
					t.Fatal(err)
				}
			}
			if next, err := Acquire(context.Background(), dir, "cycle"); err == nil {
				next.Close()
				t.Fatal("unsafe lock admitted")
			}
		})
	}
}

// A real Git process blocks on a FIFO while the creating process disappears.
func TestLeaseSurvivesOwnerDeathUntilNativeChildExits(t *testing.T) {
	for _, mode := range []string{"owner-killed", "owner-exits", "descendant", "descendant-cancelled"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "leases")
			fifo := filepath.Join(root, "input")
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
			commandCtx, cancelCommand := context.WithCancel(ctx)
			defer cancelCommand()
			cmd := exec.CommandContext(commandCtx, os.Args[0], "-test.run=^TestProcessLeaseHelper$")
			cmd.Env = append(os.Environ(), "ISSUEOPS_LEASE_HELPER="+mode, "ISSUEOPS_LEASE_DIR="+dir, "ISSUEOPS_LEASE_FIFO="+fifo)
			var lease *Lease
			if strings.HasPrefix(mode, "descendant") {
				lease, err = Acquire(ctx, dir, "cycle")
				if err != nil {
					t.Fatal(err)
				}
				defer lease.Close()
				Attach(lease.Context(ctx), cmd)
			}
			output, err := cmd.StdoutPipe()
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
			if err := json.NewDecoder(bufio.NewReader(output)).Decode(&ready); err != nil || ready.PID <= 0 {
				t.Fatalf("helper readiness=%+v err=%v", ready, err)
			}
			if mode == "owner-killed" {
				if err := cmd.Process.Kill(); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "descendant-cancelled" {
				cancelCommand()
			}
			waitErr := cmd.Wait()
			if mode != "owner-killed" && mode != "descendant-cancelled" && waitErr != nil {
				t.Fatal(waitErr)
			}
			if strings.HasPrefix(mode, "descendant") {
				if next, err := lease.Drain(ctx); !errors.Is(err, ErrBusy) {
					if next != nil {
						next.Close()
					}
					t.Fatalf("live descendant drained: %v", err)
				}
			}
			if next, err := Acquire(ctx, dir, "cycle"); !errors.Is(err, ErrBusy) {
				if next != nil {
					next.Close()
				}
				t.Fatalf("native child lost ownership: %v", err)
			}
			if err := input.Close(); err != nil {
				t.Fatal(err)
			}
			until := time.Now().Add(5 * time.Second)
			for {
				next, err := Acquire(ctx, dir, "cycle")
				if err == nil {
					next.Close()
					break
				}
				if !errors.Is(err, ErrBusy) || time.Now().After(until) {
					t.Fatalf("child exit did not release lock: %v", err)
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
}

func TestProcessLeaseHelper(t *testing.T) {
	mode := os.Getenv("ISSUEOPS_LEASE_HELPER")
	if mode == "" {
		return
	}
	ctx := context.Background()
	input, err := os.Open(os.Getenv("ISSUEOPS_LEASE_FIFO"))
	if err != nil {
		panic(err)
	}
	cmd := exec.Command("git", "hash-object", "--stdin")
	cmd.Stdin = input
	if strings.HasPrefix(mode, "descendant") {
		cmd.ExtraFiles = []*os.File{os.NewFile(3, "inherited-lease")}
	} else {
		lease, err := Acquire(ctx, os.Getenv("ISSUEOPS_LEASE_DIR"), "cycle")
		if err != nil {
			panic(err)
		}
		Attach(lease.Context(ctx), cmd)
	}
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]int{"pid": cmd.Process.Pid}); err != nil {
		panic(err)
	}
	if mode == "owner-killed" || mode == "descendant-cancelled" {
		_ = cmd.Wait()
	}
	os.Exit(0)
}
