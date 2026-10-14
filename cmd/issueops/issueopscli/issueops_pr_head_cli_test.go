package issueopscli

import (
	"strings"
	"testing"

	issueopscore "issueops/internal/adapter/issueops"
	preflight "issueops/internal/adapter/preflight"
	issueopscontract "issueops/internal/contract/issueops"
)

// A code fix after the cycle entered pr must pass the same strict pr gate again before
// pr_head (the head completion compares against) moves; until then the old head stays.
func TestCLIIssueOpsPRReentryResealsPRHeadOnlyAfterTheStrictGate(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIGitRepoForRemoteVerifyTest(t)
	started, err := startIssueOpsFixture(issueOpsStateRootForTest(), issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: "75-remote-verify-cli"})
	if err != nil {
		t.Fatal(err)
	}
	record, actor := makeIssueOpsPRPhaseRecordForCLITest(t, started.ID, repo)
	worktree := record.WorktreePath
	head := func() string {
		_, out, _ := preflight.GitCmd(worktree, "rev-parse", "HEAD")
		return strings.TrimSpace(out)
	}
	sealed := head()
	if record.PRHead != sealed {
		t.Fatalf("pr entry pr_head = %q, want worktree HEAD %q", record.PRHead, sealed)
	}

	writeIssueOpsCLIFileForTest(t, worktree, "internal/demo.go", "package demo\n\nvar fixed = true\n")
	for _, args := range [][]string{{"add", "internal/demo.go"}, {"commit", "-q", "-m", "fix: post-pr change"}, {"push", "-q"}} {
		if code, _, stderr := preflight.GitCmd(worktree, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	if _, _, err := advanceLoopPhaseForTest(issueOpsStateRootForTest(), started.ID, string(issueopscontract.IssueOpsPhasePR), actor); err == nil || !strings.Contains(err.Error(), "cannot enter pr phase") {
		t.Fatalf("reseal with stale seals must be refused, err=%v", err)
	}
	stored, err := issueopscore.ReadIssueOps(issueOpsStateRootForTest(), started.ID)
	if err != nil || stored.PRHead != sealed {
		t.Fatalf("refused reseal moved pr_head to %q (err=%v), want %q", stored.PRHead, err, sealed)
	}

	if _, _, err := advanceLoopPhaseForTest(issueOpsStateRootForTest(), started.ID, string(issueopscontract.IssueOpsPhaseAISlopClean), actor); err != nil {
		t.Fatal(err)
	}
	recordIssueOpsCoreProjectDocsReviewForCLITest(t, started.ID)
	recordIssueOpsCoreImplementationReviewForCLITest(t, started.ID)
	resealed, _, err := advanceLoopPhaseForTest(issueOpsStateRootForTest(), started.ID, string(issueopscontract.IssueOpsPhasePR), actor)
	if err != nil {
		t.Fatal(err)
	}
	if resealed.Phase != issueopscontract.IssueOpsPhasePR || resealed.PRHead != head() || resealed.PRHead == sealed {
		t.Fatalf("reseal pr_head = %q phase=%s, want new HEAD %q", resealed.PRHead, resealed.Phase, head())
	}
}
