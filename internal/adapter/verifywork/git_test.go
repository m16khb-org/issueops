package verifywork

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"issueops/internal/testsupport"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestGitStatus(t *testing.T) {
	write := func(t *testing.T, dir, name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string)
		want  string
	}{
		{"clean repo is empty", func(t *testing.T, dir string) {
			write(t, dir, "a.txt", "a")
			gitIn(t, dir, "add", "a.txt")
			gitIn(t, dir, "commit", "-m", "init")
		}, ""},
		{"untracked file", func(t *testing.T, dir string) { write(t, dir, "new.txt", "x") }, "?? new.txt\n"},
		{"staged and modified", func(t *testing.T, dir string) {
			write(t, dir, "a.txt", "a")
			gitIn(t, dir, "add", "a.txt")
			gitIn(t, dir, "commit", "-m", "init")
			write(t, dir, "a.txt", "changed")
			write(t, dir, "b.txt", "b")
			gitIn(t, dir, "add", "b.txt")
		}, " M a.txt\nA  b.txt\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testsupport.IsolateGitConfig(t)
			dir := t.TempDir()
			gitIn(t, dir, "init", "-q")
			tt.setup(t, dir)
			got, err := GitStatus(dir)
			if err != nil {
				t.Fatalf("GitStatus: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestGitStatusNotARepoReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := GitStatus(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("expected error for non-repository path")
	}
}
