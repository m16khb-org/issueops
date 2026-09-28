package issueopsapp

import (
	"context"
	"reflect"
	"strings"
	"testing"

	core "issueops/internal/adapter/issueops"
	model "issueops/internal/contract/issueops"
)

func TestBranchPrepareCompositionAdoptsOnceAndSealsResolvedCommit(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	root, repo := core.IssueOpsStateRoot(), makeGitRepoForContract(t)
	runGitForContract(t, repo, "remote", "add", "origin", "https://github.com/acme/code.git")
	head := strings.TrimSpace(claimWiringGit(t, repo, "rev-parse", "HEAD"))
	record, err := core.StartIssueOps(root, model.IssueOpsStartRequest{Repo: repo})
	if err != nil {
		t.Fatal(err)
	}
	store := core.CycleRecordStore{StateRoot: root}
	err = store.WithinLock(context.Background(), record.ID, func() error {
		record.IssueURL = "https://github.com/acme/planning/issues/63"
		var saveErr error
		record, saveErr = store.Save(record)
		return saveErr
	})
	if err != nil {
		t.Fatal(err)
	}
	service := newBranchPreparer(root)
	req := model.IssueOpsBranchPrepareRequest{Branch: "63-prepared", BaseBranch: "main", BaseSHA: "HEAD", LinkVerified: true}
	prepared, err := service.Prepare(context.Background(), record.ID, req, nil)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := core.ReadIssueOps(root, record.ID)
	if err != nil || !reflect.DeepEqual(prepared, stored) {
		t.Fatalf("stored preparation differs: %v", err)
	}
	if stored.ID != record.ID || stored.Branch != req.Branch || stored.BranchPrepare.BaseSHA != head || stored.BranchPrepare.CodeProjectKey != "github.com/acme/code" || stored.BranchPrepare.Provider != "github" || len(stored.BranchPrepare.Steps) == 0 {
		t.Fatalf("preparation=%+v", stored.BranchPrepare)
	}
	for _, invalid := range []model.IssueOpsBranchPrepareRequest{
		{Branch: "63-other", BaseBranch: "main"},
		{Branch: req.Branch, BaseBranch: "main", BaseSHA: "missing-ref"},
		{Branch: req.Branch, BaseBranch: "main", IssueURL: "https://github.com/acme/other/issues/63"},
	} {
		if _, err := service.Prepare(context.Background(), record.ID, invalid, nil); err == nil {
			t.Fatalf("invalid preparation accepted: %+v", invalid)
		}
		after, err := core.ReadIssueOps(root, record.ID)
		if err != nil || !reflect.DeepEqual(after, stored) {
			t.Fatalf("failed preparation changed persisted state: %v", err)
		}
	}
}
