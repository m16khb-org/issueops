package issueops

import (
	"os"
	"strings"
)

// CleanupStatusEnvironment gives cleanup status read-only filesystem and git
// observations. RunGit returns exit code, stdout and stderr; ReadGit returns trimmed
// output only.
type CleanupStatusEnvironment struct {
	RunGit  func(string, ...string) (int, string, string)
	ReadGit func(string, ...string) string
}

// DirectoryExists reports whether path is an existing directory. Blank paths and paths
// containing a NUL byte are never directories.
func (CleanupStatusEnvironment) DirectoryExists(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "\x00") {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// Git runs git in path and returns the exit code, stdout and stderr.
func (e CleanupStatusEnvironment) Git(path string, args ...string) (int, string, string) {
	return e.RunGit(path, args...)
}

// GitOutput runs git in path through ReadGit and returns its output.
func (e CleanupStatusEnvironment) GitOutput(path string, args ...string) string {
	return e.ReadGit(path, args...)
}
