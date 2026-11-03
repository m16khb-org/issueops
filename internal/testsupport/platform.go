package testsupport

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// AbsPath returns a synthetic absolute path for fixtures that are never
// touched on disk: /a/b on Unix and C:\a\b on Windows, so production checks
// such as filepath.IsAbs accept it on every OS.
func AbsPath(parts ...string) string {
	root := string(filepath.Separator)
	if runtime.GOOS == "windows" {
		root = `C:\`
	}
	return filepath.Join(append([]string{root}, parts...)...)
}

// SetHome points the user home directory at dir for the test: HOME on Unix,
// and USERPROFILE as well on Windows, where os.UserHomeDir reads it.
func SetHome(t testing.TB, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
	}
}

// ExecutableName returns the file name an executable called base has on this
// OS (base.exe on Windows).
func ExecutableName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

// WriteFakeCommand installs a POSIX shell script as the command name in dir
// and returns the path to run. On Unix it is the script itself, executable.
// On Windows the script is name.sh and a small native shim name.exe runs it
// with Git for Windows bash, so exec.LookPath and exec.Command find it like a
// real command. Tests that need Git Bash skip when it is not installed.
func WriteFakeCommand(t testing.TB, dir, name, script string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	bash := gitBash(t)
	shim := fakeCommandShim(t)
	base := filepath.Join(dir, strings.TrimSuffix(name, ".exe"))
	if err := os.WriteFile(base+".sh", []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	// The shim passes argv in the NUL-separated file named by FAKECMD_ARGS;
	// restore it and run the script, keeping the script path as $0.
	runner := "mapfile -d '' -t fakecmd_args < \"$FAKECMD_ARGS\"\nexec bash \"${BASH_SOURCE[0]%.run.sh}.sh\" \"${fakecmd_args[@]}\"\n"
	if err := os.WriteFile(base+".run.sh", []byte(runner), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(shim)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(base+".exe", data, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(base+".exe.bash", []byte(bash), 0o644); err != nil {
		t.Fatal(err)
	}
	return base + ".exe"
}

var gitBashPath = sync.OnceValues(func() (string, error) {
	git, err := exec.LookPath("git")
	if err != nil {
		return "", err
	}
	// Git for Windows ships git.exe in cmd\ or bin\ and bash.exe in bin\.
	root := filepath.Dir(filepath.Dir(git))
	for _, candidate := range []string{filepath.Join(root, "bin", "bash.exe"), filepath.Join(root, "usr", "bin", "bash.exe")} {
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate, nil
		}
	}
	return "", errors.New("Git for Windows bash.exe not found next to " + git)
})

func gitBash(t testing.TB) string {
	t.Helper()
	path, err := gitBashPath()
	if err != nil {
		t.Skipf("fake shell commands need Git for Windows bash: %v", err)
	}
	return path
}

var fakeCommandShimPath = sync.OnceValues(func() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("locate testsupport source")
	}
	source := filepath.Join(filepath.Dir(file), "fakecmd")
	data, err := os.ReadFile(filepath.Join(source, "main.go"))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	out := filepath.Join(os.TempDir(), "issueops-fakecmd-"+hex.EncodeToString(sum[:8])+".exe")
	if _, err := os.Stat(out); err == nil {
		return out, nil
	}
	temporary, err := os.CreateTemp(os.TempDir(), "issueops-fakecmd-*.exe")
	if err != nil {
		return "", err
	}
	temporary.Close()
	build := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-o", temporary.Name(), ".")
	build.Dir = source
	if output, err := build.CombinedOutput(); err != nil {
		os.Remove(temporary.Name())
		return "", errors.New("build fake command shim: " + err.Error() + "\n" + string(output))
	}
	// Another test process may have published it first; either copy works.
	if err := os.Rename(temporary.Name(), out); err != nil {
		os.Remove(temporary.Name())
		if _, statErr := os.Stat(out); statErr != nil {
			return "", err
		}
	}
	return out, nil
})

func fakeCommandShim(t testing.TB) string {
	t.Helper()
	path, err := fakeCommandShimPath()
	if err != nil {
		t.Fatal(err)
	}
	return path
}
