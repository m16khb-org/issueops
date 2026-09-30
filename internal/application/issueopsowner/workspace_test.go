package issueopsowner

import (
	"encoding/json"
	model "issueops/internal/contract/issueops"
	prep "issueops/internal/contract/issueopspreparation"
	"strings"
	"testing"
)

type workspaceFiles struct {
	*ownerFiles
	matches     bool
	comparisons int
}

func (f *workspaceFiles) SamePath(a, b string) bool { f.comparisons++; return f.matches }
func TestWorkspaceResolutionPreservesValidationBeforePathObservation(t *testing.T) {
	for _, tc := range []struct {
		name, branch, base, parent string
		delegated, match           bool
		want                       string
		calls                      int
	}{
		{name: "canonical parent", branch: "feature/one", base: "feature/parent", parent: "/source.worktrees/feature-parent", match: true, calls: 1},
		{name: "derived parent", branch: "feature/one", base: "feature/parent", delegated: true, match: true},
		{name: "wrong parent", branch: "feature/one", base: "main", parent: "/elsewhere", want: "does not match canonical parent", calls: 1},
		{name: "invalid branch", branch: "..", base: "main", parent: "/elsewhere", want: "execution branch is invalid"},
		{name: "invalid parent branch", branch: "feature", base: "..", parent: "/elsewhere", want: "parent execution base branch is invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, files, r, _, _, _ := ownerFixture(t)
			f := &workspaceFiles{ownerFiles: files, matches: tc.match}
			s.Files = f
			r.Branch = tc.branch
			r.BranchPrepare = &model.IssueOpsBranchPrepare{BaseSHA: "abc", BaseBranch: tc.base, ParentWorktree: tc.parent}
			if tc.delegated {
				r.Delegation = &model.IssueOpsDelegationContract{ParentCycleID: "parent"}
			}
			raw, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			got, err := s.ResolveWorkspace(prep.Snapshot{RecordRaw: raw}, true)
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("expected %q got %v", tc.want, err)
				}
			} else {
				if err != nil || got.Root != "/source.worktrees/feature-one" || got.ParentWorktree != "/source.worktrees/feature-parent" || !got.Confirm {
					t.Fatalf("workspace: %+v %v", got, err)
				}
			}
			if f.comparisons != tc.calls {
				t.Fatalf("path observations=%d want=%d", f.comparisons, tc.calls)
			}
		})
	}
}
func TestDirectMaterializationProjectsWorkspaceWithoutOwnerEffects(t *testing.T) {
	s, files, _, snapshot, _, _ := ownerFixture(t)
	err := s.MaterializeDirect(snapshot, prep.WorkspaceReceipt{SourceRoot: "/source", Root: "/direct", Branch: "direct", BaseHead: "abc", Driver: "git"})
	if err != nil || strings.Join(files.events, ",") != "materialize" || len(files.records) != 1 {
		t.Fatalf("direct effects: %v %v", err, files.events)
	}
	r := files.records[0]
	if r.WorktreePath != "/direct" || r.Execution.Mode != model.ExecutionModeDirect || r.Execution.Workspace.Root != "/direct" || r.Execution.Workspace.ArtifactDir != ".issueops/issues/480/artifact" {
		t.Fatalf("direct workspace lost identity: %+v", r)
	}
}
