package testsupport

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"
)

// WriteHangingCommand installs a fake command name (see WriteFakeCommand) in a
// new directory and returns the directory. The command writes the marker file
// "started", loops until "stop" exists, and writes "exited" when it leaves the
// loop, so it never exits on its own; a test stops it through a context.
//
// The cleanup is registered after t.TempDir, so it runs before the directory
// is removed: it creates "stop" and, on Windows, waits up to 10 s for "exited"
// once "started" exists. There the process a context kills is the fakecmd
// shim, and the bash it started keeps running and holding the script files
// until it sees "stop"; a shim killed before it started bash leaves no
// "started". Elsewhere the script itself is killed and never writes "exited".
func WriteHangingCommand(t testing.TB, name string) string {
	t.Helper()
	dir := t.TempDir()
	slash := filepath.ToSlash(dir)
	script := "#!/bin/sh\n" +
		": > '" + slash + "/started'\n" +
		"while [ ! -e '" + slash + "/stop' ]; do sleep 0.05; done\n" +
		": > '" + slash + "/exited'\n"
	WriteFakeCommand(t, dir, name, script)
	t.Cleanup(func() {
		if !stopBlockingCommand(t, dir) {
			return
		}
		if runtime.GOOS == "windows" && WaitForFile(filepath.Join(dir, "started"), time.Second) {
			WaitForFile(filepath.Join(dir, "exited"), 10*time.Second)
		}
	})
	return dir
}

// WriteLingeringCommand installs a fake command name in a new directory and
// returns the directory. The command prints output (which must not contain a
// single quote) on stdout, starts a background child that inherits stdout, and
// exits with exitCode at once. The child writes "started", keeps stdout open
// until "stop" exists, and writes "exited". A caller reading the command's
// output therefore sees the command exit while the pipe stays open.
//
// The cleanup, registered after t.TempDir, creates "stop" and waits up to 10 s
// for "exited" on every OS: the child is never killed, and it must not write
// into the directory while the directory is removed.
func WriteLingeringCommand(t testing.TB, name, output string, exitCode int) string {
	t.Helper()
	dir := t.TempDir()
	slash := filepath.ToSlash(dir)
	script := "#!/bin/sh\n" +
		"printf '%s\\n' '" + output + "'\n" +
		"( : > '" + slash + "/started'; while [ ! -e '" + slash + "/stop' ]; do sleep 0.05; done; : > '" + slash + "/exited' ) &\n" +
		"exit " + strconv.Itoa(exitCode) + "\n"
	WriteFakeCommand(t, dir, name, script)
	t.Cleanup(func() {
		if !stopBlockingCommand(t, dir) {
			return
		}
		if WaitForFile(filepath.Join(dir, "started"), time.Second) {
			WaitForFile(filepath.Join(dir, "exited"), 10*time.Second)
		}
	})
	return dir
}

// WaitForFile reports whether path exists within limit, polling every 20 ms.
func WaitForFile(path string, limit time.Duration) bool {
	deadline := time.Now().Add(limit)
	for {
		if _, err := os.Stat(path); err == nil {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// stopBlockingCommand creates the "stop" sentinel in dir.
func stopBlockingCommand(t testing.TB, dir string) bool {
	if err := os.WriteFile(filepath.Join(dir, "stop"), nil, 0o600); err != nil {
		t.Errorf("create stop sentinel: %v", err)
		return false
	}
	return true
}
