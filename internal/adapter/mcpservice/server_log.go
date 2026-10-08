//go:build darwin || linux

package mcpservice

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"
)

// serviceLogLimit bounds server.log; with the one rotated copy the log uses
// about twice this much disk.
const serviceLogLimit = 8 << 20

// ServiceLogPath is the file the launchd and systemd units append the server's
// stdout and stderr to.
func ServiceLogPath(stateDir string) string { return filepath.Join(httpDir(stateDir), "server.log") }

// ServiceLogWriter wraps stderr with a size cap when the supervisor pointed
// stderr at the service log; any other stderr (a terminal, a pipe) is returned
// unchanged. The unit files keep redirecting output, so the cap rotates by
// copy-then-truncate, which needs O_APPEND on every descriptor sharing the file.
func ServiceLogWriter(stdout, stderr *os.File, stateDir string) io.Writer {
	return serviceLogWriter(stdout, stderr, ServiceLogPath(stateDir), serviceLogLimit)
}

func serviceLogWriter(stdout, stderr *os.File, path string, limit int64) io.Writer {
	if !sameFile(stderr, path) {
		return stderr
	}
	for _, file := range []*os.File{stdout, stderr} {
		if file != nil && sameFile(file, path) {
			if err := ensureAppend(file); err != nil {
				return stderr
			}
		}
	}
	return &cappedLogWriter{file: stderr, path: path, limit: limit}
}

func sameFile(file *os.File, path string) bool {
	opened, err := file.Stat()
	if err != nil {
		return false
	}
	named, err := os.Stat(path)
	return err == nil && os.SameFile(opened, named)
}

func ensureAppend(file *os.File) error {
	fd := int(file.Fd())
	flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
	if err != nil || flags&unix.O_APPEND != 0 {
		return err
	}
	_, err = unix.FcntlInt(uintptr(fd), unix.F_SETFL, flags|unix.O_APPEND)
	return err
}

type cappedLogWriter struct {
	mu    sync.Mutex
	file  *os.File
	path  string
	limit int64
}

func (w *cappedLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.file.Write(p)
	if err != nil {
		return n, err
	}
	if info, statErr := w.file.Stat(); statErr == nil && info.Size() > w.limit {
		if rotateErr := w.rotate(); rotateErr != nil {
			fmt.Fprintf(w.file, "issueops mcp http log rotation failed: %v\n", rotateErr)
		}
	}
	return n, nil
}

// rotate copies the log to path.1 and truncates the original in place, so the
// supervisor's open descriptors keep appending to the same file.
func (w *cappedLogWriter) rotate() error {
	src, err := os.Open(w.path)
	if err != nil {
		return err
	}
	defer src.Close()
	tmp := w.path + ".1.tmp"
	dst, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, w.path+".1"); err != nil {
		return err
	}
	return w.file.Truncate(0)
}
