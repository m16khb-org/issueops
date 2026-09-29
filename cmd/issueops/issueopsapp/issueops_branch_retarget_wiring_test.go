package issueopsapp

import (
	"context"
	"reflect"
	"strings"
	"testing"

	core "issueops/internal/adapter/issueops"
	model "issueops/internal/contract/issueops"
)

func TestBranchRetargetCompositionPersistsObservedTargetAndForkPoint(t *testing.T) {
	root, repo := t.TempDir(), t.TempDir()
	record, err := startIssueOpsFixture(root, model.IssueOpsStartRequest{Repo: repo, Branch: "51-child"})
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = "https://github.com/acme/repo/issues/51"
	record.BranchPrepare = &model.IssueOpsBranchPrepare{Provider: "github", IssueURL: record.IssueURL, Branch: record.Branch, BaseBranch: "main", BaseSHA: strings.Repeat("a", 40), LinkVerified: true}
	record.RemoteArtifact = &model.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/52", TargetBranch: "main", VerifiedAt: "2026-09-28T00:00:00Z"}
	store := core.CycleRecordStore{StateRoot: root}
	if err := store.WithinLock(context.Background(), record.ID, func() error { var saveErr error; record, saveErr = store.Save(record); return saveErr }); err != nil {
		t.Fatal(err)
	}
	previousGit := core.GitCmd
	t.Cleanup(func() { core.GitCmd = previousGit })
	originCalls := 0
	core.GitCmd = func(dir string, args ...string) (int, string, string) {
		if dir != repo || !reflect.DeepEqual(args, []string{"ls-remote", "--heads", "origin", "refs/heads/50-parent"}) {
			t.Fatalf("unexpected Git request: %s %v", dir, args)
		}
		originCalls++
		return 0, strings.Repeat("b", 40) + "\trefs/heads/50-parent\n", ""
	}
	providerCalls := 0
	service := newBranchRetargeter(root, func(artifact model.IssueOpsRemoteArtifactVerification) (string, error) {
		providerCalls++
		if artifact.URL != record.RemoteArtifact.URL {
			t.Fatalf("unexpected artifact: %+v", artifact)
		}
		return "50-parent", nil
	})
	updated, err := service.Retarget(context.Background(), record.ID, model.IssueOpsBranchRetargetRequest{BaseBranch: "50-parent", Reason: "merged into parent"}, model.IssueOpsActor{})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := core.ReadIssueOps(root, record.ID)
	if err != nil || !reflect.DeepEqual(updated, stored) {
		t.Fatalf("saved retarget differs: %v", err)
	}
	if providerCalls != 1 || originCalls != 1 || stored.BranchPrepare.BaseBranch != "50-parent" || stored.RemoteArtifact.TargetBranch != "50-parent" || stored.BranchPrepare.BaseSHA != record.BranchPrepare.BaseSHA || len(stored.BranchPrepare.Retargets) != 1 {
		t.Fatalf("retarget: %+v, provider=%d origin=%d", stored.BranchPrepare, providerCalls, originCalls)
	}
}
