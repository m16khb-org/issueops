//go:build unix

package sqlstore

import (
	"path/filepath"
	"syscall"
	"testing"
)

func TestTouchPrivateCreatesOwnerReadWriteUnderRestrictiveUmask(t *testing.T) {
	path := filepath.Join(t.TempDir(), "issueops.db")
	previous := syscall.Umask(0o277)
	defer syscall.Umask(previous)
	if err := touchPrivate(path); err != nil {
		t.Fatal(err)
	}
	assertMode(t, path, 0o600)
}
