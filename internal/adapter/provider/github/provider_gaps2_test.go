package github

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestMergePullRequestRejectsBadURLMethodAndSurfacesGhFailures(t *testing.T) {
	const url = "https://github.com/acme/repo/pull/5"
	repo := fakeGhRepo(t, "#!/bin/sh\necho \"$*\" >> calls\nexit 1\n")
	p := NewProvider()
	if _, err := p.MergePullRequest(t.Context(), port.IssueProviderPullRequestMergeRequest{Repo: repo, URL: "https://github.com/acme/repo/issues/5", HeadOID: "abc", Method: model.RemoteMergeMethodSquash}); err == nil || !strings.Contains(err.Error(), "canonical") {
		t.Fatalf("err = %v", err)
	}
	if _, err := p.MergePullRequest(t.Context(), port.IssueProviderPullRequestMergeRequest{Repo: repo, URL: url, HeadOID: "abc", Method: "fast-forward"}); err == nil || !strings.Contains(err.Error(), "squash, merge, or rebase") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "calls")); !os.IsNotExist(err) {
		t.Fatal("gh must not run for rejected requests")
	}
	if _, err := p.MergePullRequest(t.Context(), port.IssueProviderPullRequestMergeRequest{Repo: repo, URL: url, HeadOID: "abc", Method: model.RemoteMergeMethodRebase, MarkReady: true}); err == nil || !strings.Contains(err.Error(), "gh pr ready failed") {
		t.Fatalf("err = %v", err)
	}
	calls, _ := os.ReadFile(filepath.Join(repo, "calls"))
	if strings.Contains(string(calls), "pr merge") {
		t.Fatalf("merge must not run after ready failed: %q", calls)
	}
}

func TestMergePullRequestMergeFailureSkipsReadback(t *testing.T) {
	repo := fakeGhRepo(t, "#!/bin/sh\necho \"$1 $2\" >> calls\nif [ \"$2\" = merge ]; then echo protected >&2; exit 1; fi\nexit 0\n")
	_, err := NewProvider().MergePullRequest(t.Context(), port.IssueProviderPullRequestMergeRequest{Repo: repo, URL: "https://github.com/acme/repo/pull/5", HeadOID: "abc", Method: model.RemoteMergeMethodMerge})
	if err == nil || !strings.Contains(err.Error(), "gh pr merge failed") {
		t.Fatalf("err = %v", err)
	}
	calls, _ := os.ReadFile(filepath.Join(repo, "calls"))
	if strings.Contains(string(calls), "pr view") {
		t.Fatalf("readback ran after failed merge: %q", calls)
	}
}

func TestClosePullRequestConfirmClosesAndVerifies(t *testing.T) {
	repo := fakeGhRepo(t, `#!/bin/sh
echo "$1 $2" >> calls
if [ "$1 $2" = "pr view" ]; then
 if [ -f closed ]; then printf '{"state":"CLOSED"}'; else printf '{"state":"OPEN"}'; fi
 exit 0
fi
if [ "$1 $2" = "pr close" ]; then : > closed; exit 0; fi
exit 2
`)
	res, err := NewProvider().ClosePullRequest(t.Context(), port.IssueProviderClosePullRequestRequest{Repo: repo, ArtifactURL: "https://github.com/acme/repo/pull/5", Confirm: true})
	if err != nil || !res.OK || !res.Closed || res.AlreadyClosed || res.Merged || res.State != "CLOSED" {
		t.Fatalf("res %+v err %v", res, err)
	}
	calls, _ := os.ReadFile(filepath.Join(repo, "calls"))
	if string(calls) != "pr view\npr close\npr view\n" {
		t.Fatalf("calls = %q", calls)
	}
}

func TestClosePullRequestFailsWhenReadbackStillOpenOrCloseFails(t *testing.T) {
	const url = "https://github.com/acme/repo/pull/5"
	repo := fakeGhRepo(t, "#!/bin/sh\nif [ \"$2\" = view ]; then printf '{\"state\":\"OPEN\"}'; fi\nexit 0\n")
	res, err := NewProvider().ClosePullRequest(t.Context(), port.IssueProviderClosePullRequestRequest{Repo: repo, ArtifactURL: url, Confirm: true})
	if err == nil || !strings.Contains(err.Error(), "not verified: state=OPEN") || res.OK || res.Closed {
		t.Fatalf("res %+v err %v", res, err)
	}
	repo = fakeGhRepo(t, "#!/bin/sh\nif [ \"$2\" = view ]; then printf '{\"state\":\"OPEN\"}'; exit 0; fi\nexit 1\n")
	if _, err := NewProvider().ClosePullRequest(t.Context(), port.IssueProviderClosePullRequestRequest{Repo: repo, ArtifactURL: url, Confirm: true}); err == nil || !strings.Contains(err.Error(), "gh pr close failed") {
		t.Fatalf("err = %v", err)
	}
	repo = fakeGhRepo(t, "#!/bin/sh\nprintf 'junk'\n")
	if _, err := NewProvider().ClosePullRequest(t.Context(), port.IssueProviderClosePullRequestRequest{Repo: repo, ArtifactURL: url, Confirm: true}); err == nil || !strings.Contains(err.Error(), "parse gh pull request state") {
		t.Fatalf("err = %v", err)
	}
	repo = fakeGhRepo(t, "#!/bin/sh\nexit 1\n")
	if _, err := NewProvider().ClosePullRequest(t.Context(), port.IssueProviderClosePullRequestRequest{Repo: repo, ArtifactURL: url, Confirm: true}); err == nil || !strings.Contains(err.Error(), "gh pr view failed") {
		t.Fatalf("err = %v", err)
	}
}

func TestCloseIssueRejectsInvalidInputAndReadbackFailures(t *testing.T) {
	const url = "https://github.com/acme/repo/issues/12"
	p := NewProvider()
	if _, err := p.CloseIssue(t.Context(), port.IssueProviderCloseIssueRequest{IssueURL: " "}); err == nil || !strings.Contains(err.Error(), "issue url is required") {
		t.Fatalf("err = %v", err)
	}
	if _, err := p.CloseIssue(t.Context(), port.IssueProviderCloseIssueRequest{IssueURL: url, Reason: "wontfix"}); err == nil || !strings.Contains(err.Error(), "completed or not_planned") {
		t.Fatalf("err = %v", err)
	}
	repo := fakeGhRepo(t, "#!/bin/sh\nexit 1\n")
	if _, err := p.CloseIssue(t.Context(), port.IssueProviderCloseIssueRequest{Repo: repo, IssueURL: url, Confirm: true}); err == nil || !strings.Contains(err.Error(), "gh issue view failed") {
		t.Fatalf("err = %v", err)
	}
	repo = fakeGhRepo(t, "#!/bin/sh\nprintf 'junk'\n")
	if _, err := p.CloseIssue(t.Context(), port.IssueProviderCloseIssueRequest{Repo: repo, IssueURL: url, Confirm: true}); err == nil || !strings.Contains(err.Error(), "parse gh issue state") {
		t.Fatalf("err = %v", err)
	}
	repo = fakeGhRepo(t, "#!/bin/sh\nif [ \"$2\" = view ]; then printf '{\"state\":\"OPEN\"}'; exit 0; fi\nexit 1\n")
	if _, err := p.CloseIssue(t.Context(), port.IssueProviderCloseIssueRequest{Repo: repo, IssueURL: url, Confirm: true}); err == nil || !strings.Contains(err.Error(), "gh issue close failed") {
		t.Fatalf("err = %v", err)
	}
}

func TestCloseIssuePassesNotPlannedReasonAndQuotesPreview(t *testing.T) {
	const url = "https://github.com/acme/repo/issues/12"
	repo := fakeGhRepo(t, `#!/bin/sh
if [ "$2" = view ]; then
 if [ -f closed ]; then printf '{"state":"CLOSED"}'; else printf '{"state":"OPEN"}'; fi; exit 0
fi
printf '%s\n' "$@" > close.argv; : > closed
`)
	p := NewProvider()
	preview, err := p.CloseIssue(t.Context(), port.IssueProviderCloseIssueRequest{Repo: repo, IssueURL: url, Reason: "not_planned"})
	if err != nil || !strings.Contains(preview.Preview, `--reason "not planned"`) {
		t.Fatalf("preview %+v err %v", preview, err)
	}
	res, err := p.CloseIssue(t.Context(), port.IssueProviderCloseIssueRequest{Repo: repo, IssueURL: url, Reason: "Not Planned", Confirm: true})
	if err != nil || !res.Closed || res.AlreadyClosed {
		t.Fatalf("res %+v err %v", res, err)
	}
	argv, _ := os.ReadFile(filepath.Join(repo, "close.argv"))
	if string(argv) != "issue\nclose\n"+url+"\n--reason\nnot planned\n" {
		t.Fatalf("argv = %q", argv)
	}
}

func TestReplaceArtifactBodyRejectsBadInputAndSurfacesGhStderr(t *testing.T) {
	const url = "https://github.com/acme/repo/issues/7"
	p := NewProvider()
	if _, err := p.ReplaceArtifactBody(t.Context(), port.IssueProviderReplaceArtifactBodyRequest{Kind: "wiki", URL: url, Body: "b"}); err == nil || !strings.Contains(err.Error(), "unsupported github artifact kind") {
		t.Fatalf("err = %v", err)
	}
	if _, err := p.ReplaceArtifactBody(t.Context(), port.IssueProviderReplaceArtifactBodyRequest{Kind: "issue", URL: " ", Body: "b"}); err == nil || !strings.Contains(err.Error(), "artifact url is required") {
		t.Fatalf("err = %v", err)
	}
	if _, err := p.ReplaceArtifactBody(t.Context(), port.IssueProviderReplaceArtifactBodyRequest{Kind: "issue", URL: url, Body: " \n"}); err == nil || !strings.Contains(err.Error(), "replacement body is empty") {
		t.Fatalf("err = %v", err)
	}
	repo := fakeGhRepo(t, "#!/bin/sh\ncat >/dev/null\nexit 1\n")
	if _, err := p.ReplaceArtifactBody(t.Context(), port.IssueProviderReplaceArtifactBodyRequest{Repo: repo, Kind: "issue", URL: url, Body: "b", Confirm: true}); err == nil || !strings.Contains(err.Error(), "gh issue edit failed") {
		t.Fatalf("err = %v", err)
	}
	repo = fakeGhRepo(t, "#!/bin/sh\nif [ \"$2\" = edit ]; then cat >/dev/null; exit 0; fi\necho 'HTTP 403 forbidden' >&2\nexit 1\n")
	if _, err := p.ReplaceArtifactBody(t.Context(), port.IssueProviderReplaceArtifactBodyRequest{Repo: repo, Kind: "issue", URL: url, Body: "b", Confirm: true}); err == nil || !strings.Contains(err.Error(), "HTTP 403 forbidden") {
		t.Fatalf("view stderr must reach caller, got %v", err)
	}
	repo = fakeGhRepo(t, "#!/bin/sh\nif [ \"$2\" = edit ]; then cat >/dev/null; exit 0; fi\nprintf 'junk'\n")
	if _, err := p.ReplaceArtifactBody(t.Context(), port.IssueProviderReplaceArtifactBodyRequest{Repo: repo, Kind: "issue", URL: url, Body: "b", Confirm: true}); err == nil || !strings.Contains(err.Error(), "parse gh issue body") {
		t.Fatalf("err = %v", err)
	}
}

func TestReadArtifactBodyRejectsBlankURLAndMissingGh(t *testing.T) {
	p := NewProvider()
	if _, err := p.ReadArtifactBody(t.Context(), port.IssueProviderArtifactBodyRequest{Kind: "issue", URL: "  "}); err == nil || !strings.Contains(err.Error(), "artifact url is required") {
		t.Fatalf("err = %v", err)
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := p.ReadArtifactBody(t.Context(), port.IssueProviderArtifactBodyRequest{Kind: "issue", URL: "https://github.com/acme/repo/issues/7"}); err == nil || !strings.Contains(err.Error(), "gh CLI is not installed") {
		t.Fatalf("err = %v", err)
	}
}

func TestVerifyChildHierarchyRejectsMalformedURLsAndGhFailure(t *testing.T) {
	repo := fakeGhRepo(t, "#!/bin/sh\nexit 1\n")
	p := NewProvider()
	if _, err := p.VerifyChildHierarchy(t.Context(), port.IssueProviderChildHierarchyRequest{Repo: repo, ParentIssueURL: "https://gitlab.com/a/b/-/issues/1", ChildURL: "https://github.com/acme/repo/issues/2"}); err == nil {
		t.Fatal("non-GitHub parent must be rejected")
	}
	if _, err := p.VerifyChildHierarchy(t.Context(), port.IssueProviderChildHierarchyRequest{Repo: repo, ParentIssueURL: gapParentURL, ChildURL: "https://github.com/acme/repo/pull/2"}); err == nil || !strings.Contains(err.Error(), "child_url must be a GitHub issue URL") {
		t.Fatalf("err = %v", err)
	}
	if res, err := p.VerifyChildHierarchy(t.Context(), port.IssueProviderChildHierarchyRequest{Repo: repo, ParentIssueURL: gapParentURL, ChildURL: "https://github.com/acme/repo/issues/2"}); err == nil || res.OK {
		t.Fatalf("gh failure must surface: %+v %v", res, err)
	}
}

func TestCloseChildRejectsForeignProjectAndUnattachedChild(t *testing.T) {
	p := NewProvider()
	if _, err := p.CloseChild(port.IssueProviderCloseChildRequest{ParentIssueURL: gapParentURL, ChildURL: "https://github.com/other/repo/issues/3", Confirm: true}); err == nil || !strings.Contains(err.Error(), "must match linked parent issue project") {
		t.Fatalf("err = %v", err)
	}
	repo := fakeGhRepo(t, "#!/bin/sh\necho \"$*\" >> calls\nprintf '[{\"id\":1,\"number\":5,\"state\":\"open\"}]'\n")
	if _, err := p.CloseChild(port.IssueProviderCloseChildRequest{Repo: repo, ParentIssueURL: gapParentURL, ChildURL: "https://github.com/acme/repo/issues/3", Confirm: true}); err == nil || !strings.Contains(err.Error(), "hierarchy verification failed") {
		t.Fatalf("err = %v", err)
	}
	calls, _ := os.ReadFile(filepath.Join(repo, "calls"))
	if strings.Contains(string(calls), "PATCH") {
		t.Fatalf("must not patch an unattached child: %q", calls)
	}
}

func TestCloseChildAlreadyClosedSkipsPatchAndFailsOnStaleReadback(t *testing.T) {
	childJSON := func(state string) string {
		return `{"id":1,"number":3,"state":"` + state + `","html_url":"https://github.com/acme/repo/issues/3"}`
	}
	script := func(listState, verifyState string) string {
		return `#!/bin/sh
echo "$*" >> calls
case "$*" in
 *sub_issues*) printf '%s' '[` + childJSON(listState) + `]';;
 *PATCH*) printf '%s' '` + childJSON("closed") + `';;
 *) printf '%s' '` + childJSON(verifyState) + `';;
esac
`
	}
	req := func(repo string) port.IssueProviderCloseChildRequest {
		return port.IssueProviderCloseChildRequest{Repo: repo, ParentIssueURL: gapParentURL, ChildURL: "https://github.com/acme/repo/issues/3", Confirm: true}
	}
	repo := fakeGhRepo(t, script("closed", "closed"))
	res, err := NewProvider().CloseChild(req(repo))
	if err != nil || !res.AlreadyClosed || !res.Closed || !res.HierarchyVerified {
		t.Fatalf("res %+v err %v", res, err)
	}
	if calls, _ := os.ReadFile(filepath.Join(repo, "calls")); strings.Contains(string(calls), "PATCH") {
		t.Fatalf("already-closed child was patched: %q", calls)
	}
	repo = fakeGhRepo(t, script("open", "open"))
	if _, err := NewProvider().CloseChild(req(repo)); err == nil || !strings.Contains(err.Error(), "close verification failed: state=open") {
		t.Fatalf("err = %v", err)
	}
}

func TestCloseChildPreviewObservesRemoteStateBestEffort(t *testing.T) {
	req := func(repo string) port.IssueProviderCloseChildRequest {
		return port.IssueProviderCloseChildRequest{Repo: repo, ParentIssueURL: gapParentURL, ChildURL: " https://github.com/acme/repo/issues/3 "}
	}
	repo := fakeGhRepo(t, `#!/bin/sh
echo "$*" >> calls
printf '%s' '[{"id":1,"number":3,"state":"CLOSED","html_url":"https://github.com/acme/repo/issues/3"}]'
`)
	res, err := NewProvider().CloseChild(req(repo))
	if err != nil || !res.OK || !res.HierarchyVerified || !res.AlreadyClosed || res.State != "CLOSED" || res.Closed {
		t.Fatalf("res %+v err %v", res, err)
	}
	if calls, _ := os.ReadFile(filepath.Join(repo, "calls")); strings.Contains(string(calls), "PATCH") {
		t.Fatalf("preview must be read-only: %q", calls)
	}
	repo = fakeGhRepo(t, "#!/bin/sh\nexit 1\n")
	res, err = NewProvider().CloseChild(req(repo))
	if err != nil || !res.OK || res.HierarchyVerified || res.State != "" || res.ChildURL != "https://github.com/acme/repo/issues/3" {
		t.Fatalf("observation failure must leave state unknown: %+v err %v", res, err)
	}
}

func TestParseGitHubIssueURLRejectsIncompleteAndNonGitHubURLs(t *testing.T) {
	for _, raw := range []string{"", "https://gitlab.com/a/b/issues/1", "https://github.com//b/issues/1", "https://github.com/a/b/pull/1", "https://github.com/a/b/issues/"} {
		if _, _, _, err := parseGitHubIssueURL(raw); err == nil {
			t.Fatalf("%q must be rejected", raw)
		}
	}
	o, r, n, err := parseGitHubIssueURL(" https://github.com/a/b/issues/9 ")
	if err != nil || o != "a" || r != "b" || n != "9" {
		t.Fatalf("got %q %q %q %v", o, r, n, err)
	}
}
