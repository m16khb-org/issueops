//go:build darwin

package cmux

import (
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestObserveEndpointReportsExactSocketCTimeNanoseconds(t *testing.T) {
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
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	stat := info.Sys().(*syscall.Stat_t)
	want := int64(stat.Ctimespec.Sec)*1_000_000_000 + int64(stat.Ctimespec.Nsec)
	if got.CTimeNS != want {
		t.Fatalf("CTimeNS=%d want %d", got.CTimeNS, want)
	}
}
