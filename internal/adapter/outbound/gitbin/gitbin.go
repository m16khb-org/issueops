// Package gitbin names the git executable the adapters run.
//
// On macOS /usr/bin/git is an xcrun shim: every call first resolves the active
// developer directory and then execs <developer dir>/usr/bin/git, which costs
// about 5 ms per spawn (measured 8.5 ms through the shim, 3.0 ms direct). A
// single `issueops next` spawns git about fourteen times. Path resolves the
// same binary the shim would exec and returns it, so the result is the same
// git without the detour. Anything else on PATH is returned unchanged.
package gitbin

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const (
	macShim        = "/usr/bin/git"
	xcodeSelectLnk = "/var/db/xcode_select_link"
)

// Path returns the git executable to run. It resolves PATH on every call (as
// exec.Command does), so a caller or test that changes PATH is honoured.
func Path() string {
	found, err := exec.LookPath("git")
	if err != nil {
		return "git"
	}
	if runtime.GOOS != "darwin" || filepath.Clean(found) != macShim {
		return found
	}
	if direct := shimTarget(os.Getenv, os.Readlink, executable); direct != "" {
		return direct
	}
	return found
}

// shimTarget mirrors xcrun's choice of developer directory: DEVELOPER_DIR,
// then the xcode-select link. When neither names a git binary it returns ""
// and the caller keeps the shim.
func shimTarget(getenv func(string) string, readlink func(string) (string, error), isExecutable func(string) bool) string {
	dir := getenv("DEVELOPER_DIR")
	if dir == "" {
		link, err := readlink(xcodeSelectLnk)
		if err != nil {
			return ""
		}
		dir = link
	}
	candidate := filepath.Join(dir, "usr", "bin", "git")
	if !isExecutable(candidate) {
		return ""
	}
	return candidate
}

func executable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}
