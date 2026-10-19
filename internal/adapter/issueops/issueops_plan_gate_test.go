package issueops

import (
	reviewcontract "issueops/internal/contract/issueopsreview"
	"strings"
	"testing"

	"issueops/internal/adapter/preflight"
	"issueops/internal/contract/issueops"
)

// B4 plan-before-execute gate: AdvanceIssueOpsPhase(...,"implement") must block
// on architecture_approval when the architecture review is not approved. The gate already
// exists (issueOpsBaseImplementationMissing -> issueOpsArchitectureReviewMissing), and
// is exercised via LinkIssueOpsPlan and the full-lifecycle test; this asserts
// the DIRECT advance-to-implement boundary specifically, and that recording an
// approved architecture review clears the architecture_approval block.
func TestAdvanceToImplementGatesOnArchitectureApproval(t *testing.T) {
	t.Parallel()

	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	branch := "1-demo"
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "-b", branch); code != 0 {
		t.Fatalf("git checkout branch failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "push", "-q", "-u", "origin", branch); code != 0 {
		t.Fatalf("git push branch failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "main"); code != 0 {
		t.Fatalf("git checkout main failed: %s", stderr)
	}
	worktree := issueOpsWorktreePathForTest(repo, "1-demo")
	if code, _, stderr := preflight.GitCmd(repo, "worktree", "add", "-q", worktree, branch); code != 0 {
		t.Fatalf("git worktree add failed: %s", stderr)
	}

	record, err := startIssueOpsFixture(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsIntentForTest(t, stateRoot, record.ID)
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(issueops.IssueOpsPhaseGrill)); err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/1"); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareIssueOpsBranch(stateRoot, record.ID, issueops.IssueOpsBranchPrepareRequest{
		Provider: "github", IssueURL: "https://github.com/example/repo/issues/1",
		Branch: branch, BaseBranch: "main", LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsWorktree(stateRoot, record.ID, worktree); err != nil {
		t.Fatal(err)
	}
	setIssueOpsPlanPrepForTest(t, stateRoot, record.ID)

	// No architecture review recorded: implement entry is blocked because the review
	// is missing entirely.
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(issueops.IssueOpsPhaseImplement)); err == nil || !strings.Contains(err.Error(), "architecture_review") {
		t.Fatalf("implement entry with no architecture review must be blocked on architecture_review, got %v", err)
	}

	// An UNAPPROVED architecture review: implement entry must be blocked SPECIFICALLY on
	// architecture_approval (the plan-before-execute gate — a plan exists/was reviewed
	// but not approved).
	if _, err := RecordIssueOpsArchitectureReview(stateRoot, record.ID, reviewcontract.ArchitectureReviewRequest{
		ProblemSummary:       "IssueOps must preserve the work contract",
		ProposedArchitecture: "Gate implementation on a reviewed design contract",
		Verification:         []string{"architecture review checked alternatives and risks"},
		Approved:             false,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(issueops.IssueOpsPhaseImplement)); err == nil || !strings.Contains(err.Error(), "architecture_approval") {
		t.Fatalf("implement entry with an unapproved architecture review must be blocked on architecture_approval, got %v", err)
	}

	// Recording an approved architecture review clears the architecture_approval block (other
	// readiness items such as the plan may remain, but the approval gate is gone).
	recordIssueOpsApprovedArchitectureForTest(t, stateRoot, record.ID)
	if _, err := AdvanceIssueOpsPhase(stateRoot, record.ID, string(issueops.IssueOpsPhaseImplement)); err != nil && strings.Contains(err.Error(), "architecture_approval") {
		t.Fatalf("approved architecture review must clear the architecture_approval block, still got %v", err)
	}
}
