package issueopsowner

import (
	"encoding/json"
	"errors"
	model "issueops/internal/contract/issueops"
	prep "issueops/internal/contract/issueopspreparation"
	"strings"
	"testing"
)

type workspaceFiles struct {
	matches      bool
	comparisons  int
	materialized []model.IssueOpsRecord
	err          error
}

func (f *workspaceFiles) SamePath(a, b string) bool { f.comparisons++; return f.matches }
func (f *workspaceFiles) Materialize(record model.IssueOpsRecord) (map[string]string, error) {
	f.materialized = append(f.materialized, record)
	return nil, f.err
}

func ownerRecord() model.IssueOpsRecord {
	return model.IssueOpsRecord{ID: "io-owner", Repo: "/source", IssueURL: "https://github.com/acme/repo/issues/480", Branch: "feature/one"}
}

func rawSnapshot(t *testing.T, record model.IssueOpsRecord) prep.Snapshot {
	t.Helper()
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	return prep.Snapshot{RecordRaw: raw}
}

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
			f := &workspaceFiles{matches: tc.match}
			s := Service{Files: f}
			r := ownerRecord()
			r.Branch = tc.branch
			r.BranchPrepare = &model.IssueOpsBranchPrepare{BaseSHA: "abc", BaseBranch: tc.base, ParentWorktree: tc.parent}
			if tc.delegated {
				r.Delegation = &model.IssueOpsDelegationContract{ParentCycleID: "parent"}
			}
			got, err := s.ResolveWorkspace(rawSnapshot(t, r), true)
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

func TestDirectMaterializationProjectsWorkspaceIdentity(t *testing.T) {
	f := &workspaceFiles{}
	s := Service{Files: f}
	err := s.MaterializeDirect(rawSnapshot(t, ownerRecord()), prep.WorkspaceReceipt{SourceRoot: "/source", Root: "/direct", Branch: "direct", BaseHead: "abc"})
	if err != nil || len(f.materialized) != 1 {
		t.Fatalf("direct effects: %v %v", err, f.materialized)
	}
	r := f.materialized[0]
	if r.WorktreePath != "/direct" || r.Execution.Workspace.Root != "/direct" || r.Execution.Workspace.ArtifactDir != ".issueops/issues/480/artifact" {
		t.Fatalf("direct workspace lost identity: %+v", r)
	}
}

func TestWorkspaceHelpersRejectCorruptRawRecord(t *testing.T) {
	s := Service{Files: &workspaceFiles{}}
	if _, err := s.ResolveWorkspace(prep.Snapshot{RecordRaw: []byte("{")}, true); err == nil {
		t.Fatal("ResolveWorkspace must reject corrupt raw record")
	}
	if err := s.MaterializeDirect(prep.Snapshot{}, prep.WorkspaceReceipt{}); err == nil {
		t.Fatal("MaterializeDirect must reject missing raw record")
	}
}

func TestMaterializeDirectPropagatesMaterializeFailure(t *testing.T) {
	boom := errors.New("cannot copy")
	s := Service{Files: &workspaceFiles{err: boom}}
	if err := s.MaterializeDirect(rawSnapshot(t, ownerRecord()), prep.WorkspaceReceipt{Root: "/wt"}); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
}

func TestOwnerArtifactDirFallsBackToBranchPrepareIssue(t *testing.T) {
	for name, tc := range map[string]struct {
		record model.IssueOpsRecord
		want   string
	}{
		"record issue wins": {model.IssueOpsRecord{IssueURL: "https://github.com/a/b/issues/7", BranchPrepare: &model.IssueOpsBranchPrepare{IssueURL: "https://github.com/a/b/issues/8"}}, ".issueops/issues/7/artifact"},
		"branch issue":      {model.IssueOpsRecord{IssueURL: "not-an-issue", BranchPrepare: &model.IssueOpsBranchPrepare{IssueURL: "https://github.com/a/b/issues/8"}}, ".issueops/issues/8/artifact"},
		"neither":           {model.IssueOpsRecord{IssueURL: "not-an-issue"}, ""},
		"branch unnumbered": {model.IssueOpsRecord{BranchPrepare: &model.IssueOpsBranchPrepare{IssueURL: "nope"}}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			if got := OwnerArtifactDir(tc.record); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
