package issueops

import (
	"path/filepath"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/testsupport"
)

var layoutRepo = testsupport.AbsPath("work", "repo")

func layoutRecord() model.IssueOpsRecord {
	return model.IssueOpsRecord{
		ID: "io-1", Repo: layoutRepo, IssueURL: "https://github.com/acme/repo/issues/7", Branch: "feat/x",
		Phase: model.IssueOpsPhasePlan,
		BranchPrepare: &model.IssueOpsBranchPrepare{
			Provider: "github", IssueURL: "https://github.com/acme/repo/issues/7", Branch: "feat/x", BaseBranch: "main", BaseSHA: "abc123",
		},
	}
}

func TestDeriveWorkspaceLayoutDerivesCanonicalWorktreePaths(t *testing.T) {
	record := layoutRecord()
	worktrees := layoutRepo + ".worktrees"
	epic := filepath.Join(worktrees, "epic-one")
	layout, err := DeriveWorkspaceLayout(record)
	if err != nil {
		t.Fatal(err)
	}
	want := WorkspaceLayout{Root: filepath.Join(worktrees, "feat-x"), Branch: "feat/x", BaseBranch: "main", BaseHead: "abc123"}
	if layout != want {
		t.Fatalf("layout=%+v want=%+v", layout, want)
	}

	sep := string(filepath.Separator)
	record.BranchPrepare.BaseBranch = "epic/one"
	record.BranchPrepare.ParentWorktree = worktrees + sep + ".." + sep + "repo.worktrees" + sep + "epic-one"
	layout, err = DeriveWorkspaceLayout(record)
	if err != nil {
		t.Fatal(err)
	}
	if layout.ParentWorktree != epic || layout.ExpectedParent != epic {
		t.Fatalf("explicit parent layout=%+v", layout)
	}
	elsewhere := testsupport.AbsPath("elsewhere", "epic-one")
	record.BranchPrepare.ParentWorktree = elsewhere
	layout, err = DeriveWorkspaceLayout(record)
	if err != nil || layout.ParentWorktree != elsewhere || layout.ExpectedParent != epic {
		t.Fatalf("non-canonical parent must surface the expected one: %+v %v", layout, err)
	}

	record.BranchPrepare.ParentWorktree = ""
	record.Delegation = &model.IssueOpsDelegationContract{ParentCycleID: "parent"}
	layout, err = DeriveWorkspaceLayout(record)
	if err != nil || layout.ParentWorktree != epic || layout.ExpectedParent != "" {
		t.Fatalf("delegated cycle must derive its parent worktree: %+v %v", layout, err)
	}
	record.Delegation = &model.IssueOpsDelegationContract{ParentCycleID: "  "}
	layout, err = DeriveWorkspaceLayout(record)
	if err != nil || layout.ParentWorktree != "" {
		t.Fatalf("blank parent cycle id must not count as delegated: %+v %v", layout, err)
	}
}

func TestDeriveWorkspaceLayoutRejectsUnsafeInputs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*model.IssueOpsRecord)
		want   string
	}{
		{"no branch prepare", func(r *model.IssueOpsRecord) { r.BranchPrepare = nil }, "base_sha is required"},
		{"blank base sha", func(r *model.IssueOpsRecord) { r.BranchPrepare.BaseSHA = "  " }, "base_sha is required"},
		{"empty branch", func(r *model.IssueOpsRecord) { r.Branch = " " }, "execution branch is invalid"},
		{"dot branch", func(r *model.IssueOpsRecord) { r.Branch = "." }, "execution branch is invalid"},
		{"dotdot branch", func(r *model.IssueOpsRecord) { r.Branch = ".." }, "execution branch is invalid"},
		{"parent without base branch", func(r *model.IssueOpsRecord) { r.BranchPrepare.ParentWorktree = "/p"; r.BranchPrepare.BaseBranch = "" }, "base branch is invalid"},
		{"delegated with dotdot base", func(r *model.IssueOpsRecord) {
			r.Delegation = &model.IssueOpsDelegationContract{ParentCycleID: "x"}
			r.BranchPrepare.BaseBranch = ".."
		}, "base branch is invalid"},
	} {
		record := layoutRecord()
		tc.mutate(&record)
		if _, err := DeriveWorkspaceLayout(record); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: err=%v", tc.name, err)
		}
	}
}

func TestValidateWorkspaceParentReportsBothPaths(t *testing.T) {
	layout := WorkspaceLayout{ParentWorktree: "/actual", ExpectedParent: "/canonical"}
	if err := ValidateWorkspaceParent(layout, true); err != nil {
		t.Fatal(err)
	}
	err := ValidateWorkspaceParent(layout, false)
	if err == nil || !strings.Contains(err.Error(), `"/actual"`) || !strings.Contains(err.Error(), `"/canonical"`) {
		t.Fatalf("err=%v", err)
	}
}
