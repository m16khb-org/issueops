package issueops

import (
	"path/filepath"
	"testing"

	"issueops/internal/testsupport"
)

func TestCycleIdentitiesKeepTheirGitObserver(t *testing.T) {
	original := GitCmd
	t.Cleanup(func() { GitCmd = original })
	worktree := testsupport.AbsPath("worktree")
	first, second := testsupport.AbsPath("first"), testsupport.AbsPath("second")
	GitCmd = func(string, ...string) (int, string, string) { return 0, filepath.Join(first, ".git"), "" }
	firstIdentity := CycleStartIdentity{RunGit: GitCmd}
	GitCmd = func(string, ...string) (int, string, string) { return 0, filepath.Join(second, ".git"), "" }
	secondIdentity := CycleStartIdentity{RunGit: GitCmd}
	if got := firstIdentity.CanonicalRepo(worktree); got != first {
		t.Fatalf("first identity=%q want %s", got, first)
	}
	if got := secondIdentity.CanonicalRepo(worktree); got != second {
		t.Fatalf("second identity=%q want %s", got, second)
	}
}

func TestSourceRootCleansRelativeCallerPath(t *testing.T) {
	abs, err := filepath.Abs("repo/../repo")
	if err != nil {
		t.Fatalf("filepath.Abs failed: %v", err)
	}
	if got := (CycleStartIdentity{}).CanonicalRepo("repo/../repo"); got != abs {
		t.Fatalf("SourceRoot relative path = %q, want cleaned absolute %q", got, abs)
	}
}
