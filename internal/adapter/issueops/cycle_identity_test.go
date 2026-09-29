package issueops

import (
	"path/filepath"
	"testing"
)

func TestCycleIdentitiesKeepTheirGitObserver(t *testing.T) {
	original := GitCmd
	t.Cleanup(func() { GitCmd = original })
	GitCmd = func(string, ...string) (int, string, string) { return 0, "/first/.git", "" }
	first := CycleStartIdentity{RunGit: GitCmd}
	GitCmd = func(string, ...string) (int, string, string) { return 0, "/second/.git", "" }
	second := CycleStartIdentity{RunGit: GitCmd}
	if got := first.CanonicalRepo("/worktree"); got != "/first" {
		t.Fatalf("first identity=%q want /first", got)
	}
	if got := second.CanonicalRepo("/worktree"); got != "/second" {
		t.Fatalf("second identity=%q want /second", got)
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
