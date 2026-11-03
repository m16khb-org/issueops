//go:build darwin || linux

package cmux

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ObserveEndpoint pins a Unix socket's owner and mode; cmux is unsupported on
// windows, where there is no such endpoint.

func TestObserveEndpointPinsPrivateUnixSocketAndParent(t *testing.T) {
	dir := shortTempDir(t)
	path := filepath.Join(dir, "cmux.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ObserveEndpoint(path, os.Getuid())
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != path || got.ParentPath != dir || got.Kind != "unix_socket" || got.Inode == 0 || got.CTimeNS <= 0 || got.Mode != 0o600 {
		t.Fatalf("endpoint=%+v", got)
	}
	if err := ValidateEndpoint(got, path, os.Getuid()); err != nil {
		t.Fatalf("validate observed endpoint: %v", err)
	}
}

func TestObserveEndpointRejectsMissingSymlinkAndUnsafeMode(t *testing.T) {
	dir := shortTempDir(t)
	missing := filepath.Join(dir, "missing.sock")
	if _, err := ObserveEndpoint(missing, os.Getuid()); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("missing socket error=%v", err)
	}

	target := filepath.Join(dir, "target.sock")
	listener, err := net.Listen("unix", target)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	if err := os.Chmod(target, 0o600); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(dir, "link.sock")
	if err := os.Symlink(target, symlink); err != nil {
		t.Fatal(err)
	}
	if _, err := ObserveEndpoint(symlink, os.Getuid()); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink socket error=%v", err)
	}

	if err := os.Chmod(target, 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := ObserveEndpoint(target, os.Getuid()); err == nil || !strings.Contains(err.Error(), "0600") {
		t.Fatalf("unsafe socket error=%v", err)
	}
}

func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "io-cmux-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}
