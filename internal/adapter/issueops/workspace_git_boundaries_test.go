package issueops

import (
	"context"
	"slices"
	"strings"
	"testing"

	"issueops/internal/testsupport"
)

func TestBranchGitOriginPresentReportsAbsentForBlankListing(t *testing.T) {
	t.Parallel()

	git := BranchGit{Run: func(string, ...string) (int, string, string) { return 0, "  \n", "" }}
	found, err := git.OriginPresent("repo", "branch")
	if err != nil || found {
		t.Fatalf("OriginPresent on blank ls-remote = %v, %v; want false, nil", found, err)
	}
}

func TestObserveGitHubLinkedBranchesPassesIssueNumberContainingZero(t *testing.T) {
	t.Parallel()

	var argv []string
	observe := ObserveGitHubLinkedBranches(func(_ context.Context, _ string, args ...string) (string, error) {
		argv = args
		return `{"data":{"repository":{"issue":{"linkedBranches":{"totalCount":0,"nodes":[]}}}}}`, nil
	})
	if _, err := observe(context.Background(), "https://github.com/o/r/issues/90"); err != nil {
		t.Fatalf("issue number with a zero digit must be accepted: %v", err)
	}
	if !slices.Contains(argv, "number=90") {
		t.Fatalf("argv = %v, want number=90", argv)
	}
}

func TestOrphanEnvironmentRunReportsTrimmedGitFailure(t *testing.T) {
	testsupport.IsolateGitConfig(t)

	_, err := OrphanEnvironment{}.Run(context.Background(), t.TempDir(), "rev-parse", "--git-dir")
	if err == nil {
		t.Fatal("git outside a repository must fail")
	}
	msg := err.Error()
	if msg == "" || msg == "git returned a non-zero exit" || msg != strings.TrimSpace(msg) {
		t.Fatalf("failure detail = %q, want trimmed git stderr", msg)
	}
}

func TestBoundedGitFailureFallsBackForBlankStderrAndTrims(t *testing.T) {
	t.Parallel()

	if got := boundedGitFailure(" \n"); got != "git returned a non-zero exit" {
		t.Fatalf("blank stderr = %q", got)
	}
	if got := boundedGitFailure("  boom\n"); got != "boom" {
		t.Fatalf("stderr = %q, want trimmed boom", got)
	}
}
