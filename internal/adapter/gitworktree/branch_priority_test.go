package gitworktree

import (
	"issueops/internal/port"
	"reflect"
	"testing"
)

func TestWorktreeBranchPriorityAndShortCircuit(t *testing.T) {
	refs := []string{"refs/heads/feature", "refs/remotes/origin/feature", "refs/remotes/upstream/feature"}
	for mask := 0; mask < 8; mask++ {
		queried := []string{}
		p := Provisioner{GitOut: func(dir string, args ...string) string {
			if dir != "/source" || len(args) != 4 || !reflect.DeepEqual(args[:3], []string{"show-ref", "--verify", "--hash"}) {
				t.Fatalf("unexpected lookup %s %v", dir, args)
			}
			ref := args[3]
			queried = append(queried, ref)
			for i, value := range refs {
				if ref == value && mask&(1<<i) != 0 {
					return "oid"
				}
			}
			return ""
		}}
		got := p.worktreeAddArgs(port.ExecutionWorkspaceReceipt{SourceRoot: "/source", Root: "/source.worktrees/feature", Branch: "feature", BaseHead: "base"})
		selected := 3
		for i := 0; i < 3; i++ {
			if mask&(1<<i) != 0 {
				selected = i
				break
			}
		}
		want := []string{"worktree", "add", "-q", "-b", "feature", "/source.worktrees/feature", "base"}
		count := 3
		if selected < 3 {
			count = selected + 1
			want[len(want)-1] = refs[selected]
		}
		if selected == 0 {
			want = []string{"worktree", "add", "-q", "/source.worktrees/feature", "feature"}
		}
		if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(queried, refs[:count]) {
			t.Fatalf("availability=%03b args=%v want=%v queried=%v", mask, got, want, queried)
		}
	}
}
