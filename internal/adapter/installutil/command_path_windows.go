//go:build windows

package installutil

import (
	"errors"
	"fmt"
	"os"
)

// Adopting a regular command file swaps it for the managed symlink with one
// atomic exchange of two names. Windows has no such exchange (MoveFileEx and
// ReplaceFile each leave a window or rewrite the file), so adoption is refused
// before anything is inspected or written.
func managedCommandAdoptionSupport(path string) error {
	return fmt.Errorf("--adopt-command-file is not supported on windows, which cannot exchange two paths atomically; move %s away and run install again", path)
}

var errManagedCommandExchange = errors.New("atomic command path exchange is not supported on windows")

func exchangeManagedCommandPaths(string, string) error { return errManagedCommandExchange }

// managedCommandStat is reached only after adoption support passed, which never
// happens on Windows.
func managedCommandStat(os.FileInfo) (managedCommandStatFields, bool) {
	return managedCommandStatFields{}, false
}
