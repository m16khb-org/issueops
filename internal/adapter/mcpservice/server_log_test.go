//go:build darwin || linux

package mcpservice

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func openServiceLogForTest(t *testing.T, path string, flags int) *os.File {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|flags, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}

func TestCappedLogWriterCopyTruncatesPastLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.log")
	stderr := openServiceLogForTest(t, path, 0)
	stdout := openServiceLogForTest(t, path, os.O_APPEND)
	writer := serviceLogWriter(stdout, stderr, path, 16)
	if _, ok := writer.(*cappedLogWriter); !ok {
		t.Fatalf("service log writer = %T", writer)
	}
	if flags, err := unix.FcntlInt(stderr.Fd(), unix.F_GETFL, 0); err != nil || flags&unix.O_APPEND == 0 {
		t.Fatalf("stderr flags=%#x err=%v, want O_APPEND", flags, err)
	}
	for _, line := range []string{"first line\n", "second line\n", "third\n"} {
		if _, err := writer.Write([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := stdout.WriteString("raw\n"); err != nil {
		t.Fatal(err)
	}
	rotated, err := os.ReadFile(path + ".1")
	if err != nil || string(rotated) != "first line\nsecond line\n" {
		t.Fatalf("rotated=%q err=%v", rotated, err)
	}
	current, err := os.ReadFile(path)
	if err != nil || string(current) != "third\nraw\n" {
		t.Fatalf("current=%q err=%v, want appends after truncation without a hole", current, err)
	}
	if _, err := os.Stat(path + ".1.tmp"); !os.IsNotExist(err) {
		t.Fatalf("temporary rotation file left behind: %v", err)
	}
}

func TestServiceLogWriterWrapsOnlyTheServiceLog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.log")
	other := openServiceLogForTest(t, filepath.Join(dir, "other.log"), os.O_APPEND)
	if writer := serviceLogWriter(nil, other, path, 16); writer != other {
		t.Fatalf("missing service log wrapped %T", writer)
	}
	openServiceLogForTest(t, path, os.O_APPEND)
	if writer := serviceLogWriter(nil, other, path, 16); writer != other {
		t.Fatalf("unrelated stderr wrapped %T", writer)
	}
	if got := ServiceLogPath("/state"); !strings.HasSuffix(got, filepath.Join("mcp-http", "server.log")) {
		t.Fatalf("service log path = %q", got)
	}
}

func TestServiceLogWriterWrapsStderrPointedAtTheStateDirLog(t *testing.T) {
	state := t.TempDir()
	if err := os.MkdirAll(httpDir(state), 0o700); err != nil {
		t.Fatal(err)
	}
	stderr := openServiceLogForTest(t, ServiceLogPath(state), 0)
	if _, ok := ServiceLogWriter(nil, stderr, state).(*cappedLogWriter); !ok {
		t.Fatal("stderr redirected to the service log was not capped")
	}
	other := openServiceLogForTest(t, filepath.Join(t.TempDir(), "term"), 0)
	if writer := ServiceLogWriter(nil, other, state); writer != other {
		t.Fatalf("foreign stderr wrapped as %T", writer)
	}
}

func TestCappedLogWriterReportsRotationFailureInTheLogAndKeepsWriting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.log")
	stderr := openServiceLogForTest(t, path, os.O_APPEND)
	writer := serviceLogWriter(nil, stderr, path, 4)
	moved := filepath.Join(dir, "moved.log")
	if err := os.Rename(path, moved); err != nil {
		t.Fatal(err)
	}
	if n, err := writer.Write([]byte("well past the limit\n")); err != nil || n != len("well past the limit\n") {
		t.Fatalf("n=%d err=%v; a failed rotation must not fail the write", n, err)
	}
	got, err := os.ReadFile(moved)
	if err != nil || !strings.Contains(string(got), "well past the limit\n") || !strings.Contains(string(got), "log rotation failed") {
		t.Fatalf("log = %q err=%v", got, err)
	}
}

func TestCappedLogWriterPropagatesWriteErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.log")
	stderr := openServiceLogForTest(t, path, os.O_APPEND)
	writer := serviceLogWriter(nil, stderr, path, 1024)
	if err := stderr.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("x")); err == nil {
		t.Fatal("write to a closed log succeeded")
	}
}

func TestSameFileIsFalseForClosedFileOrMissingPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	file := openServiceLogForTest(t, path, 0)
	if !sameFile(file, path) {
		t.Fatal("open file not recognised at its own path")
	}
	if sameFile(file, path+".missing") {
		t.Fatal("matched a path that does not exist")
	}
	_ = file.Close()
	if sameFile(file, path) {
		t.Fatal("closed file matched")
	}
}
