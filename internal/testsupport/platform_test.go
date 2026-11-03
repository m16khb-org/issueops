package testsupport

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/osfile"
)

func TestWriteFakeCommandRunsByNameWithArgsStdinAndExitStatus(t *testing.T) {
	dir := t.TempDir()
	WriteFakeCommand(t, dir, "gh", `#!/bin/sh
read line
printf '%s|%s|%s|%s|%s\n' "$1" "$2" "$3" "$4" "$line"
exit 7
`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	path, err := exec.LookPath("gh")
	if err != nil || filepath.Dir(path) != dir {
		t.Fatalf("LookPath(gh) = %q, %v", path, err)
	}
	command := exec.Command("gh", "/repo/a b", `C:\x`, "@{u}", `{"q":"*.go"}`)
	command.Stdin = strings.NewReader("input\n")
	output, err := command.Output()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 7 {
		t.Fatalf("exit = %v", err)
	}
	if got := string(output); got != "/repo/a b|C:\\x|@{u}|{\"q\":\"*.go\"}|input\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestWriteFakeCommandRunsWithAnEmptyEnvironment(t *testing.T) {
	path := WriteFakeCommand(t, t.TempDir(), "probe", "#!/bin/sh\nprintf '%s' \"$1\"\n")
	command := exec.Command(path, "ok")
	command.Env = []string{}
	if output, err := command.Output(); err != nil || string(output) != "ok" {
		t.Fatalf("output = %q, %v", output, err)
	}
}

func TestMakePermissiveExposesGroupAndOtherAccess(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	if err := osfile.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "state.db")
	if err := osfile.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for path, mode := range map[string]os.FileMode{dir: 0o755, file: 0o644} {
		MakePermissive(t, path, mode)
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := osfile.Perm(path, info); perm&0o077 == 0 {
			t.Fatalf("%s perm = %o, want group/other access", path, perm)
		}
	}
}

func TestMakeUnreadableFailsReadsUntilCleanup(t *testing.T) {
	file := filepath.Join(t.TempDir(), "data")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Run("unreadable", func(t *testing.T) {
		MakeUnreadable(t, file, 0o600)
		if _, err := os.ReadFile(file); err == nil || errors.Is(err, os.ErrNotExist) {
			t.Fatalf("read err = %v, want a permission error", err)
		}
	})
	if data, err := os.ReadFile(file); err != nil || string(data) != "x" {
		t.Fatalf("after cleanup read = %q, %v", data, err)
	}
}

func TestAbsPathIsAbsolute(t *testing.T) {
	if path := AbsPath("repo", "bin", "issueops"); !filepath.IsAbs(path) || filepath.Base(path) != "issueops" {
		t.Fatalf("AbsPath = %q", path)
	}
}
