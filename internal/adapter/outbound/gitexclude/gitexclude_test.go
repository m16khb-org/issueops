package gitexclude

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A lease token written under a linked worktree's .issueops/state/ must not
// make the worktree dirty in a target repo whose .gitignore does not list it;
// otherwise every worktree_clean gate fails on harness output (an agent run
// could not abandon a cycle because of it).
func TestEnsureWorkspaceStateHidesTokensInLinkedWorktrees(t *testing.T) {
	repo := t.TempDir()
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.email=a@b", "-c", "user.name=n"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return string(out)
	}
	git(repo, "init", "-q", "-b", "main")
	git(repo, "commit", "-q", "--allow-empty", "-m", "init")
	worktree := filepath.Join(t.TempDir(), "wt")
	git(repo, "worktree", "add", "-q", "-b", "feature", worktree, "main")
	token := filepath.Join(worktree, ".issueops", "state", "issueops-v1", "k", "lease-1.token")
	if err := os.MkdirAll(filepath.Dir(token), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(token, []byte("t\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(git(worktree, "status", "--porcelain")) == "" {
		t.Fatal("fixture: the token should be untracked before the exclusion")
	}
	for range 2 {
		if err := EnsureWorkspaceState(worktree); err != nil {
			t.Fatal(err)
		}
	}
	if status := strings.TrimSpace(git(worktree, "status", "--porcelain")); status != "" {
		t.Fatalf("token still visible: %q", status)
	}
	exclude, err := os.ReadFile(filepath.Join(repo, ".git", "info", "exclude"))
	if err != nil || strings.Count(string(exclude), WorkspaceStateLine+"\n") != 1 {
		t.Fatalf("exclude must hold the line once: %q %v", exclude, err)
	}
}
