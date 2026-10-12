package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/testsupport"
)

// 실제로 빌드한 바이너리에서 VCS 세대를 읽어야 한다.
func TestFileBuildGenerationReadsVCSSettingsFromBuiltBinary(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	run := func(name string, args ...string) string {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOFLAGS=", "GOWORK=off", "GOTOOLCHAIN=local")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write("go.mod", "module fixture\n\ngo 1.21\n")
	write("main.go", "package main\n\nfunc main() {}\n")
	run("git", "init", "-q")
	run("git", "add", ".")
	run("git", "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "init")
	revision := run("git", "rev-parse", "HEAD")
	bin := filepath.Join(t.TempDir(), "fixture")
	run("go", "build", "-buildvcs=true", "-o", bin, ".")

	got := FileBuildGeneration(bin)
	if got.Revision != revision || got.Modified {
		t.Fatalf("clean build generation = %+v, want revision %s unmodified", got, revision)
	}

	write("main.go", "package main\n\nfunc main() { _ = 1 }\n")
	dirty := filepath.Join(t.TempDir(), "dirty")
	run("go", "build", "-buildvcs=true", "-o", dirty, ".")
	if got := FileBuildGeneration(dirty); got.Revision != revision || !got.Modified {
		t.Fatalf("dirty build generation = %+v", got)
	}
}
