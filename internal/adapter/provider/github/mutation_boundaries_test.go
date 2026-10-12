package github

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/port"
)

func TestCreatedArtifactNumberRejectsZeroNumber(t *testing.T) {
	for _, raw := range []string{"https://github.com/acme/repo/issues/0", "https://github.com/acme/repo/pull/0"} {
		if got := createdArtifactNumber(raw); got != "" {
			t.Fatalf("createdArtifactNumber(%q) = %q, want empty", raw, got)
		}
	}
}

func TestGitHubCreateChildRejectsParentURLWithoutNumberOrWithEmptyRepo(t *testing.T) {
	for _, parent := range []string{
		"https://github.com/acme/repo/issues",
		"https://github.com/acme//issues/5",
		"https://github.com//repo/issues/5",
	} {
		t.Run(parent, func(t *testing.T) {
			_, err := NewProvider().CreateChild(port.IssueProviderCreateChildRequest{Repo: t.TempDir(), ParentIssueURL: parent, Title: "child"})
			if err == nil || !strings.Contains(err.Error(), "parent_issue_url must be a GitHub issue URL") {
				t.Fatalf("err = %v, want parent_issue_url rejection", err)
			}
		})
	}
}

func TestGitHubCreateChildHelpWithOnlyUnrelatedFlagsNeverCreates(t *testing.T) {
	bin, repo := t.TempDir(), t.TempDir()
	writeFakeGh(t, bin, "#!/bin/sh\nif [ \"$3\" = \"--help\" ]; then echo '  --foo string'; exit 0; fi\necho create >> calls\nexit 2\n")
	t.Setenv("PATH", bin)
	_, err := NewProvider().CreateChild(port.IssueProviderCreateChildRequest{Repo: repo, ParentIssueURL: "https://github.com/acme/repo/issues/12", Title: "child", Confirm: true})
	if err == nil || !strings.Contains(err.Error(), "cannot determine gh issue create parent capability") {
		t.Fatalf("err = %v, want capability error", err)
	}
	if _, statErr := os.Stat(filepath.Join(repo, "calls")); !os.IsNotExist(statErr) {
		t.Fatal("create invoked although the help output was not recognized")
	}
}

func TestGitHubVerifyChildHierarchyDoesNotMatchIDlessEntriesWhenIDUnknown(t *testing.T) {
	binDir, repo := t.TempDir(), t.TempDir()
	writeFakeGh(t, binDir, "#!/bin/sh\nif [ \"$1\" = \"api\" ]; then printf '[{\"number\":31}]'; exit 0; fi\nexit 2\n")
	t.Setenv("PATH", binDir)
	got, err := NewProvider().VerifyChildHierarchy(t.Context(), port.IssueProviderChildHierarchyRequest{
		Repo: repo, ParentIssueURL: "https://github.com/acme/repo/issues/12", ChildURL: "https://github.com/acme/repo/issues/99",
	})
	if err != nil || got.Verified {
		t.Fatalf("verify = %+v, %v; want not verified", got, err)
	}
}

func prViewScript(headOID string) string {
	return `#!/bin/sh
if [ "$1 $2" = "pr create" ]; then printf 'https://github.com/acme/repo/pull/16\n'; exit 0; fi
if [ "$1 $2" = "pr view" ]; then
  printf '{"url":"https://github.com/acme/repo/pull/16","title":"PR","body":"","headRefName":"feat/x","baseRefName":"main","isDraft":false,"headRefOid":"` + headOID + `","labels":[],"assignees":[],"headRepository":{"nameWithOwner":"acme/repo"}}'
  exit 0
fi
exit 2
`
}

func TestGitHubCreatePullRequestExpectedHeadSHAIsOptionalButEnforcedWhenSet(t *testing.T) {
	oid := strings.Repeat("a", 40)
	for _, tc := range []struct {
		name    string
		sha     string
		wantErr string
	}{
		{"unset accepts any head", "", ""},
		{"matching", oid, ""},
		{"mismatch", strings.Repeat("b", 40), "head SHA does not match accepted final head"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin, repo := t.TempDir(), t.TempDir()
			writeFakeGh(t, bin, prViewScript(oid))
			t.Setenv("PATH", bin)
			_, err := NewProvider().CreatePullRequest(port.IssueProviderCreatePullRequestRequest{Repo: repo, ProjectKey: "github.com/acme/repo", Title: "PR", HeadBranch: "feat/x", BaseBranch: "main", ExpectedHeadSHA: tc.sha, Confirm: true})
			if tc.wantErr == "" && err != nil {
				t.Fatalf("err = %v, want success", err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestGitHubReconcileCandidateBoundIsExactlyTwo(t *testing.T) {
	row := `{"url":"https://github.com/acme/repo/pull/%d","title":"PR","body":"","headRefName":"feat/x","baseRefName":"main","isDraft":false,"headRefOid":"aaaa","labels":[],"assignees":[],"headRepository":{"nameWithOwner":"acme/repo"}}`
	for _, tc := range []struct {
		name    string
		rows    string
		want    int
		wantErr bool
	}{
		{"two", strings.ReplaceAll(row, "%d", "1") + "," + strings.ReplaceAll(row, "%d", "2"), 2, false},
		{"three", strings.ReplaceAll(row, "%d", "1") + "," + strings.ReplaceAll(row, "%d", "2") + "," + strings.ReplaceAll(row, "%d", "3"), 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin, repo := t.TempDir(), t.TempDir()
			writeFakeGh(t, bin, "#!/bin/sh\nif [ \"$1 $2\" = \"pr list\" ]; then printf '["+tc.rows+"]'; exit 0; fi\nexit 2\n")
			t.Setenv("PATH", bin)
			result, err := NewProvider().ReconcilePullRequest(port.IssueProviderReconcilePullRequestRequest{Repo: repo, ProjectKey: "github.com/acme/repo", HeadBranch: "feat/x", BaseBranch: "main"})
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "exceeded bound") {
					t.Fatalf("err = %v, want bound error", err)
				}
				return
			}
			if err != nil || len(result.Candidates) != tc.want {
				t.Fatalf("result = %#v, %v; want %d candidates", result, err, tc.want)
			}
		})
	}
}
