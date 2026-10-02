package issueopsapp

import (
	"context"
	"reflect"
	"strings"
	"testing"

	core "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/preflight"
	model "issueops/internal/contract/issueops"
)

func TestBranchRetargetCompositionPersistsObservedTargetAndForkPoint(t *testing.T) {
	root, repo := t.TempDir(), makeGitRepoForContract(t)
	remote := makeGitRepoForContract(t)
	for _, command := range []struct {
		root string
		args []string
	}{{remote, []string{"branch", "50-parent"}}, {repo, []string{"remote", "add", "origin", remote}}} {
		if code, _, stderr := preflight.GitCmd(command.root, command.args...); code != 0 {
			t.Fatal(stderr)
		}
	}
	record, err := startIssueOpsFixture(root, model.IssueOpsStartRequest{Repo: repo, Branch: "51-child"})
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = "https://github.com/acme/repo/issues/51"
	record.BranchPrepare = &model.IssueOpsBranchPrepare{Provider: "github", IssueURL: record.IssueURL, Branch: record.Branch, BaseBranch: "main", BaseSHA: strings.Repeat("a", 40), LinkVerified: true}
	record.RemoteArtifact = &model.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/52", TargetBranch: "main", VerifiedAt: "2026-09-28T00:00:00Z"}
	store := core.CycleRecordStore{StateRoot: root}
	if err := store.WithinLock(context.Background(), record.ID, func(spanCtx context.Context) error {
		var saveErr error
		record, saveErr = store.Save(spanCtx, record)
		return saveErr
	}); err != nil {
		t.Fatal(err)
	}
	providerCalls := 0
	service := newBranchRetargeter(root, func(artifact model.IssueOpsRemoteArtifactVerification) (string, error) {
		providerCalls++
		if artifact.URL != record.RemoteArtifact.URL {
			t.Fatalf("unexpected artifact: %+v", artifact)
		}
		return "50-parent", nil
	})
	t.Chdir(t.TempDir())
	originCalls := 0
	boundOrigin := service.OriginPresent
	service.OriginPresent = func(dir, branch string) (bool, error) {
		originCalls++
		if dir != repo || branch != "50-parent" {
			t.Fatalf("unexpected origin observation: %s %s", dir, branch)
		}
		return boundOrigin(dir, branch)
	}
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
