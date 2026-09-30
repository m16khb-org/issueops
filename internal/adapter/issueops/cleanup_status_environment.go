package issueops

import (
	"os"
	"strings"
)

type CleanupStatusEnvironment struct {
	RunGit  func(string, ...string) (int, string, string)
	ReadGit func(string, ...string) string
}

func (CleanupStatusEnvironment) DirectoryExists(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "\x00") {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
func (e CleanupStatusEnvironment) Git(path string, args ...string) (int, string, string) {
	return e.RunGit(path, args...)
}
func (e CleanupStatusEnvironment) GitOutput(path string, args ...string) string {
	return e.ReadGit(path, args...)
}
