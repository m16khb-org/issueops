package issueops

import (
	model "issueops/internal/contract/issueops"
	"testing"
)

func TestWorkspaceSnapshotKeepsItsGitRunner(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-b", "main"}, {"-c", "user.name=issueops", "-c", "user.email=issueops@example.invalid", "commit", "--allow-empty", "-m", "init"}} {
		if code, _, stderr := GitCmd(root, args...); code != 0 {
			t.Fatalf("git %v: %s", args, stderr)
		}
	}
	snapshot := (LeaseWorkspaceSnapshot{GitCmd: GitCmd, GitCmdRaw: GitCmdRaw}).Snapshot
	workspace := model.Workspace{Root: root, Branch: "main"}
	want, err := snapshot(workspace)
	if err != nil {
		t.Fatal(err)
	}
	original := GitCmdRaw
	t.Cleanup(func() { GitCmdRaw = original })
	GitCmdRaw = func(string, ...string) (int, string, string) { return 1, "", "another context's runner" }
	got, err := snapshot(workspace)
	if err != nil || got != want {
		t.Fatalf("prepared snapshot changed after another context configured Git: got=%q want=%q err=%v", got, want, err)
	}
}
