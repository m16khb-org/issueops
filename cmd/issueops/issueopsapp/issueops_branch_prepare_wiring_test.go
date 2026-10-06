package issueopsapp

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	core "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/outbound/sqlstore"
	model "issueops/internal/contract/issueops"
)

func TestBranchPrepareCompositionAdoptsOnceAndSealsResolvedCommit(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	root, repo := issueOpsStateRoot(), makeGitRepoForContract(t)
	runGitForContract(t, repo, "remote", "add", "origin", "https://github.com/acme/code.git")
	head := strings.TrimSpace(claimWiringGit(t, repo, "rev-parse", "HEAD"))
	record, err := startIssueOpsFixture(root, model.IssueOpsStartRequest{Repo: repo})
	if err != nil {
		t.Fatal(err)
	}
	store := core.CycleRecordStore{StateRoot: root}
	err = store.WithinLock(context.Background(), record.ID, func(spanCtx context.Context) error {
		record.IssueURL = "https://github.com/acme/planning/issues/63"
		var saveErr error
		record, saveErr = store.Save(spanCtx, record)
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

func TestBranchPrepareUsesItsExplicitStateRootForUmbrella(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	root, repo := t.TempDir(), makeGitRepoForContract(t)
	parent, err := startIssueOpsFixture(root, model.IssueOpsStartRequest{Repo: repo, Branch: "78-umbrella"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := startIssueOpsFixture(root, model.IssueOpsStartRequest{Repo: repo, Branch: "79-child"})
	if err != nil {
		t.Fatal(err)
	}
	child.IssueURL = "https://github.com/acme/repo/issues/79"
	parent.IssueLinks = []model.IssueOpsIssueLink{{Type: "child", URL: child.IssueURL, CreatedAt: parent.CreatedAt}}
	if _, err = (core.CycleRecordStore{StateRoot: root}).Save(context.Background(), parent); err != nil {
		t.Fatal(err)
	}
	child, err = (core.CycleRecordStore{StateRoot: root}).Save(context.Background(), child)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	before, found, err := db.Get("issueops_v1", child.ID)
	if err != nil || !found {
		t.Fatalf("child snapshot: found=%v err=%v", found, err)
	}
	request := model.IssueOpsBranchPrepareRequest{Branch: child.Branch, BaseBranch: "main", CodeProjectKey: "github.com/acme/repo", LinkVerified: true}
	if _, err = newBranchPreparer(root).Prepare(context.Background(), child.ID, request, nil); err == nil || !strings.Contains(err.Error(), parent.Branch) {
		t.Fatalf("explicit state-root umbrella was bypassed: %v", err)
	}
	after, found, err := db.Get("issueops_v1", child.ID)
	if err != nil || !found || !bytes.Equal(before, after) {
		t.Fatalf("rejected preparation changed the child: %v", err)
	}
	request.BaseBranch = parent.Branch
	if _, err = newBranchPreparer(root).Prepare(context.Background(), child.ID, request, nil); err != nil {
		t.Fatalf("matching local umbrella rejected: %v", err)
	}
}
