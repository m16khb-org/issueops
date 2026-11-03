package installcli

import (
	"os"
	"runtime"
	"testing"

	"issueops/internal/testsupport"
)

// isolatedPATH is the PATH install tests run with: a fixed system PATH on Unix
// that never lists the temporary home's .local/bin. Windows keeps the ambient
// PATH because Git and the shell tools it provides are resolved through it.
func isolatedPATH() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("PATH")
	}
	return "/usr/bin:/bin"
}

func captureStatusVerifyStdout(t *testing.T, fn func() error) string {
	t.Helper()
	return testsupport.CaptureStdout(t, fn)
}

// skipWithoutCommandAdoption skips tests that adopt a regular command file.
// Windows refuses --adopt-command-file because it cannot exchange two paths
// atomically; TestInstallRefusesCommandAdoptionOnWindows covers that refusal.
func skipWithoutCommandAdoption(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("--adopt-command-file is refused on Windows, which cannot exchange two paths atomically")
	}
}

// wantFixturePerm is the permission a freshly written 0o755 test executable
// reports through osfile.Perm: Windows has no group or other bits to grant, and
// the temporary directories it lives in are owner-only by inheritance.
var wantFixturePerm = func() os.FileMode {
	if runtime.GOOS == "windows" {
		return 0o700
	}
	return 0o755
}()

// previousCommandTarget is the symlink target an earlier install left behind.
var previousCommandTarget = testsupport.AbsPath("previous", "issueops")

// adoptCommandArgs prepends --adopt-command-file on every OS that supports it.
// Windows refuses the flag, so its fixtures start from the managed command
// symlink a previous install leaves instead of from a regular file to adopt.
func adoptCommandArgs(args ...string) []string {
	if runtime.GOOS == "windows" {
		return args
	}
	return append([]string{"--adopt-command-file"}, args...)
}
