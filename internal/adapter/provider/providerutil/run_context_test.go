//go:build unix

package providerutil

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestBoundedCommandStartErrorUnwrapsCause(t *testing.T) {
	_, err := RunBoundedReadbackContext(context.Background(), t.TempDir(), "issueops-no-such-executable")
	if err == nil {
		t.Fatal("expected a start failure")
	}
	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("start error does not unwrap to exec.ErrNotFound: %v", err)
	}
	if !strings.HasPrefix(err.Error(), "command start failed: ") {
		t.Fatalf("start error text changed: %q", err.Error())
	}
}

func TestRunContextStopsOnCanceledContext(t *testing.T) {
	t.Run("canceled before start", func(t *testing.T) {
		dir := t.TempDir()
		marker := filepath.Join(dir, "ran")
		script := filepath.Join(dir, "mark")
		if err := os.WriteFile(script, []byte("#!/bin/sh\n: > \"$1\"\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := RunContext(ctx, dir, script, marker); err != ctx.Err() {
			t.Fatalf("error = %v, want ctx.Err()", err)
		}
		if _, err := os.Stat(marker); err == nil {
			t.Fatal("command ran after its context was canceled")
		}
	})
	t.Run("canceled while running", func(t *testing.T) {
		dir := t.TempDir()
		fifo := filepath.Join(dir, "started")
		if err := syscall.Mkfifo(fifo, 0o600); err != nil {
			t.Fatal(err)
		}
		script := filepath.Join(dir, "slow")
		if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf started > \"$1\"\nexec sleep 30\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		started := make(chan struct{})
		go func() {
			if data, _ := os.ReadFile(fifo); string(data) == "started" {
				close(started)
			}
		}()
		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan error, 1)
		go func() {
			_, err := RunContext(ctx, dir, script, fifo)
			result <- err
		}()
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("command did not start")
		}
		cancel()
		select {
		case err := <-result:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want context.Canceled", err)
			}
			if _, isExit := errors.AsType[*exec.ExitError](err); isExit {
				t.Fatalf("error wraps the killed child's ExitError: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("canceled command did not stop")
		}
	})
	t.Run("returns stdout and the child's failure", func(t *testing.T) {
		dir := t.TempDir()
		script := filepath.Join(dir, "fail")
		if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf out\nprintf boom >&2\nexit 3\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		out, err := RunContext(context.Background(), dir, script)
		exit, isExit := errors.AsType[*exec.ExitError](err)
		if string(out) != "out" || !isExit || exit.ExitCode() != 3 || string(exit.Stderr) != "boom" {
			t.Fatalf("out=%q err=%v", out, err)
		}
	})
}
