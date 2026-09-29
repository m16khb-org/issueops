package gitworktree

import (
	"context"
	"issueops/internal/port"
	"testing"
)

func TestProvisionersKeepTheirGitObservers(t *testing.T) {
	prepare := func(repo string) Provisioner {
		read := func(dir string, args ...string) string {
			if dir != repo {
				t.Errorf("observer for %s read %s", repo, dir)
			}
			return repo
		}
		run := func(string, ...string) (int, string, string) { t.Fatal("dry run mutated Git"); return 1, "", "" }
		return Provisioner{GitCmd: run, GitOut: read}
	}
	firstRoot, secondRoot := t.TempDir(), t.TempDir()
	first, second := prepare(firstRoot), prepare(secondRoot)
	for _, tc := range []struct {
		p    Provisioner
		root string
	}{{first, firstRoot}, {second, secondRoot}, {first, firstRoot}} {
		receipt, err := tc.p.Prepare(context.Background(), port.ExecutionWorkspaceRequest{LifecycleID: "io-test", SourceRoot: tc.root, Root: tc.root + ".worktrees/test", Branch: "test", BaseHead: "base"})
		if err != nil || receipt.Exists {
			t.Fatalf("dry-run receipt=%+v err=%v", receipt, err)
		}
	}
}
