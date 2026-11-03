//go:build unix

package testsupport

import (
	"io/fs"
	"os"
	"testing"
)

// MakePermissive gives path the non-private unixMode (for example 0o644 or
// 0o755) so a test can prove production repairs or rejects it.
func MakePermissive(t testing.TB, path string, unixMode fs.FileMode) {
	t.Helper()
	if err := os.Chmod(path, unixMode); err != nil {
		t.Fatal(err)
	}
}

// MakeUnreadable removes every permission from path so reads fail with a
// permission error, and restores restoreUnixMode when the test ends.
func MakeUnreadable(t testing.TB, path string, restoreUnixMode fs.FileMode) {
	t.Helper()
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, restoreUnixMode) })
}
