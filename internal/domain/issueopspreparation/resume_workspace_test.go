package issueopspreparation

import (
	"path/filepath"
	"strings"
	"testing"

	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func TestResumeWorkspaceUsesSealedBranchPreparation(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	record := preparationcontract.Record{
		ID: "io-1", Repo: repo, Branch: "193/fix",
		BranchPrepare: []byte(`{"base_branch":"main","base_sha":"base-head"}`),
	}
	plan, err := ResumeWorkspace(record)
	if err != nil {
		t.Fatal(err)
	}
	workspace := plan.Request
	if workspace.Root != filepath.Join(repo+".worktrees", "193-fix") || workspace.BaseHead != "base-head" || workspace.BaseBranch != "main" || !workspace.Confirm {
		t.Fatalf("workspace=%+v", workspace)
	}
	record.BranchPrepare = []byte(`{"base_branch":"main"}`)
	if _, err := ResumeWorkspace(record); err == nil || !strings.Contains(err.Error(), "base_sha") {
		t.Fatalf("missing base SHA error=%v", err)
	}
}

func TestResumeWorkspaceExposesDelegatedParentForFilesystemValidation(t *testing.T) {
	record := preparationcontract.Record{
		ID: "io-1", Repo: "/repo", Branch: "193-fix",
		BranchPrepare: []byte(`{"base_branch":"main","base_sha":"base-head","parent_worktree":"/wrong"}`),
	}
	plan, err := ResumeWorkspace(record)
	if err != nil {
		t.Fatal(err)
	}
	if plan.ExpectedParent != "/repo.worktrees/main" || plan.Request.ParentWorktree != "/wrong" {
		t.Fatalf("plan=%+v", plan)
	}
}
