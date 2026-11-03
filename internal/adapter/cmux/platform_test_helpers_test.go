package cmux

import (
	"path/filepath"
	"runtime"
	"testing"

	"issueops/internal/testsupport"
)

// uncleanPath returns an absolute spelling of path that filepath.Clean would
// rewrite, for fixtures that must be rejected as "not clean".
func uncleanPath(path string) string {
	separator := string(filepath.Separator)
	return path + separator + ".." + separator + filepath.Base(path)
}

// shellCommand returns an executable and arguments that run script under a
// POSIX shell: /bin/sh -c on Unix, a Git Bash fake command on Windows.
func shellCommand(t *testing.T, script string) (string, []string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		return "/bin/sh", []string{"-c", script}
	}
	return testsupport.WriteFakeCommand(t, t.TempDir(), "runner", "#!/bin/sh\n"+script+"\n"), nil
}
