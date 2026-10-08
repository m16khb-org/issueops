package issueopsapp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHookRepoNameNamesTheSourceCheckoutOfALinkedWorktree(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "issueops")
	worktree := filepath.Join(base, "issueops.worktrees", "555-branch")
	gitdir := filepath.Join(source, ".git", "worktrees", "555-branch")
	for _, dir := range []string{worktree, gitdir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+gitdir+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitdir, "commondir"), []byte("../..\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ repo, want string }{
		{source, "issueops"},
		{worktree, "issueops"},
		{filepath.Join(base, "plain-dir"), "plain-dir"},
	} {
		if got := hookRepoName(tc.repo); got != tc.want {
			t.Fatalf("hookRepoName(%q) = %q, want %q", tc.repo, got, tc.want)
		}
	}
}

func TestHookRepoNameFallsBackToTheDirectoryWhenGitMetadataIsBroken(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "broken")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git"), []byte("gitdir: /does/not/exist\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := hookRepoName(repo); got != "broken" {
		t.Fatalf("hookRepoName = %q, want broken", got)
	}
}
