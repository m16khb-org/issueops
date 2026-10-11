package gitlab

import (
	"context"
	"errors"
	"strings"
	"testing"

	executionissue "issueops/internal/contract/executionissue"
	completionmodel "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// installGlab writes a fake glab that logs every call to glab.calls in the
// returned repo directory.
func installGlab(t *testing.T, body string) string {
	t.Helper()
	skipOnWindows(t)
	binDir, repo := t.TempDir(), t.TempDir()
	writeFakeGlab(t, binDir, "#!/bin/sh\nprintf '%s\\n' \"$*\" >> glab.calls\n"+body)
	t.Setenv("PATH", binDir)
	return repo
}

const failScript = "echo 'boom' >&2\nexit 1\n"

func TestGitLabVerifyChildHierarchyReportsMembership(t *testing.T) {
	for name, tt := range map[string]struct {
		childIID string
		want     bool
	}{
		"child in hierarchy":     {"34", true},
		"child not in hierarchy": {"35", false},
	} {
		t.Run(name, func(t *testing.T) {
			repo := installGlab(t, `case "$*" in
  *parentIid*) printf '{"data":{"project":{"issue":{"id":"gid://gitlab/WorkItem/12"}}}}'; exit 0 ;;
  *children*) printf '{"data":{"workItem":{"widgets":[{"type":"HIERARCHY","children":{"nodes":[{"id":"gid://gitlab/WorkItem/34","iid":"34","state":"OPEN"}]}}]}}}'; exit 0 ;;
esac
exit 2
`)
			got, err := NewProvider().VerifyChildHierarchy(context.Background(), port.IssueProviderChildHierarchyRequest{
				Repo: repo, ParentIssueURL: gapParentURL, ChildURL: "https://gitlab.example.com/acme/repo/-/work_items/" + tt.childIID,
			})
			if err != nil {
				t.Fatal(err)
			}
			if !got.OK || got.Provider != "gitlab" || got.Verified != tt.want {
				t.Fatalf("result = %+v, want verified=%v", got, tt.want)
			}
		})
	}
}

func TestGitLabVerifyChildHierarchyTreatsLookupFailureAsUnverified(t *testing.T) {
	repo := installGlab(t, failScript)
	got, err := NewProvider().VerifyChildHierarchy(context.Background(), port.IssueProviderChildHierarchyRequest{
		Repo: repo, ParentIssueURL: gapParentURL, ChildURL: gapChildURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Verified {
		t.Fatalf("an unreadable hierarchy must not verify: %+v", got)
	}
}

func TestGitLabVerifyChildHierarchyRejectsNonGitLabURLs(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	p := NewProvider()
	if _, err := p.VerifyChildHierarchy(context.Background(), port.IssueProviderChildHierarchyRequest{ParentIssueURL: "https://example.com/not-an-issue", ChildURL: gapChildURL}); err == nil || !strings.Contains(err.Error(), "GitLab issue URL") {
		t.Fatalf("parent error = %v", err)
	}
	if _, err := p.VerifyChildHierarchy(context.Background(), port.IssueProviderChildHierarchyRequest{ParentIssueURL: gapParentURL, ChildURL: "https://example.com/not-an-issue"}); err == nil || !strings.Contains(err.Error(), "child_url must be") {
		t.Fatalf("child error = %v", err)
	}
}

func TestGitLabArtifactBodyReadAndReplaceFailures(t *testing.T) {
	ctx := context.Background()
	issueURL := "https://gitlab.example.com/acme/repo/-/issues/12"
	t.Run("read rejects unsupported kind", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if _, err := NewProvider().ReadArtifactBody(ctx, port.IssueProviderArtifactBodyRequest{Kind: "pr", URL: issueURL}); err == nil || !strings.Contains(err.Error(), "unsupported gitlab artifact kind") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("read surfaces glab failure", func(t *testing.T) {
		repo := installGlab(t, failScript)
		if _, err := NewProvider().ReadArtifactBody(ctx, port.IssueProviderArtifactBodyRequest{Repo: repo, Kind: "issue", URL: issueURL}); err == nil || !strings.Contains(err.Error(), "glab api failed") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("read surfaces malformed payload", func(t *testing.T) {
		repo := installGlab(t, "printf 'not json'\n")
		if _, err := NewProvider().ReadArtifactBody(ctx, port.IssueProviderArtifactBodyRequest{Repo: repo, Kind: "issue", URL: issueURL}); err == nil || !strings.Contains(err.Error(), "parse glab artifact description") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("replace rejects bad kind", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if _, err := NewProvider().ReplaceArtifactBody(ctx, port.IssueProviderReplaceArtifactBodyRequest{Kind: "bogus", URL: issueURL, Body: "x"}); err == nil || !strings.Contains(err.Error(), "unsupported gitlab artifact kind") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("replace rejects blank body", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if _, err := NewProvider().ReplaceArtifactBody(ctx, port.IssueProviderReplaceArtifactBodyRequest{Kind: "issue", URL: issueURL, Body: " \n\t", Confirm: true}); err == nil || !strings.Contains(err.Error(), "replacement body is empty") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("replace rejects oversized body before any glab call", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		_, err := NewProvider().ReplaceArtifactBody(ctx, port.IssueProviderReplaceArtifactBodyRequest{Kind: "issue", URL: issueURL, Body: strings.Repeat("a", gitLabIssueBodyLimit+1), Confirm: true})
		if err == nil || !strings.Contains(err.Error(), "exceeds the GitLab description limit") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("replace accepts a body exactly at the limit in preview", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		res, err := NewProvider().ReplaceArtifactBody(ctx, port.IssueProviderReplaceArtifactBodyRequest{Kind: "issue", URL: issueURL, Body: strings.Repeat("a", gitLabIssueBodyLimit)})
		if err != nil || !res.OK {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
	t.Run("replace surfaces PUT failure", func(t *testing.T) {
		repo := installGlab(t, failScript)
		res, err := NewProvider().ReplaceArtifactBody(ctx, port.IssueProviderReplaceArtifactBodyRequest{Repo: repo, Kind: "issue", URL: issueURL, Body: "new", Confirm: true})
		if err == nil || res.Updated || res.OK {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
	t.Run("replace surfaces readback failure after PUT", func(t *testing.T) {
		repo := installGlab(t, `case "$*" in *"--method PUT"*) printf '{}'; exit 0 ;; esac
printf 'not json'
`)
		res, err := NewProvider().ReplaceArtifactBody(ctx, port.IssueProviderReplaceArtifactBodyRequest{Repo: repo, Kind: "issue", URL: issueURL, Body: "new", Confirm: true})
		if err == nil || !strings.Contains(err.Error(), "parse glab artifact description") || res.Updated {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
	t.Run("replace tolerates whitespace-only readback drift", func(t *testing.T) {
		repo := installGlab(t, `case "$*" in *"--method PUT"*) printf '{}'; exit 0 ;; esac
printf '{"description":"new\\n\\n","web_url":"https://gitlab.example.com/acme/repo/-/issues/12"}'
`)
		res, err := NewProvider().ReplaceArtifactBody(ctx, port.IssueProviderReplaceArtifactBodyRequest{Repo: repo, Kind: "issue", URL: issueURL, Body: "new", Confirm: true})
		if err != nil || !res.Updated {
			t.Fatalf("trailing newlines normalised by GitLab must verify: res=%+v err=%v", res, err)
		}
	})
}

func TestGitLabMergeRequestURLParsingAcceptsSchemelessAndRejectsOthers(t *testing.T) {
	host, project, iid, err := parseGitLabMergeRequestURL("gitlab.example.com/acme/sub/repo/-/merge_requests/9")
	if err != nil || host != "gitlab.example.com" || project != "acme/sub/repo" || iid != "9" {
		t.Fatalf("host=%q project=%q iid=%q err=%v", host, project, iid, err)
	}
	for _, raw := range []string{"", "   ", "https://gitlab.example.com/acme/repo/-/merge_requests/", "https://gitlab.example.com/acme/repo", "https://gitlab.example.com/-/merge_requests/9", "http://[::1"} {
		if _, _, _, err := parseGitLabMergeRequestURL(raw); err == nil || !strings.Contains(err.Error(), "merge request URL") {
			t.Fatalf("url %q error = %v", raw, err)
		}
	}
}

func TestGitLabSnapshotIdentityHandlesSchemelessAndUnparsableURLs(t *testing.T) {
	if !sameGitLabIssueSnapshotIdentity("gitlab.example.com/acme/repo/-/issues/1", "https://gitlab.example.com/acme/repo/-/issues/1") {
		t.Fatal("a scheme-less URL must match its https form")
	}
	if !sameGitLabIssueSnapshotIdentity("https://GitLab.Example.com/acme/repo/-/issues/1", "https://gitlab.example.com/acme/repo/-/issues/1") {
		t.Fatal("host comparison must be case-insensitive")
	}
	if sameGitLabIssueSnapshotIdentity("https://gitlab.example.com/Acme/repo/-/issues/1", "https://gitlab.example.com/acme/repo/-/issues/1") {
		t.Fatal("project paths are case-sensitive")
	}
	if sameGitLabIssueSnapshotIdentity("%zz", "%zz") || sameGitLabIssueSnapshotIdentity("", "") {
		t.Fatal("unparsable URLs must never be considered the same issue")
	}
	if got := gitLabSnapshotAuthority("%zz"); got != "" {
		t.Fatalf("authority of an unparsable URL = %q", got)
	}
	if got := gitLabSnapshotAuthority("gitlab.example.com:8443/acme/repo/-/issues/1"); got != "gitlab.example.com:8443" {
		t.Fatalf("scheme-less authority = %q", got)
	}
}

func TestGitLabReadIssueSnapshotRejectsBadInputsAndResponses(t *testing.T) {
	url := "https://gitlab.example.com/acme/repo/-/issues/69"
	t.Run("canceled context never reaches glab", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := NewProvider().ReadIssueSnapshot(ctx, executionissue.ExecutionIssueSnapshotRequest{Repo: t.TempDir(), URL: url}); !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("non issue URL", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if _, err := NewProvider().ReadIssueSnapshot(t.Context(), executionissue.ExecutionIssueSnapshotRequest{Repo: t.TempDir(), URL: "https://gitlab.example.com/acme/repo"}); err == nil {
			t.Fatal("a URL without an issue marker must be rejected")
		}
	})
	for name, tt := range map[string]struct{ body, wantErr string }{
		"glab failure":  {failScript, "glab issue snapshot read failed"},
		"bad json":      {"printf 'nope'\n", "parse glab issue snapshot"},
		"other issue":   {`printf '{"web_url":"https://gitlab.example.com/acme/repo/-/issues/70","description":"x","state":"opened"}'` + "\n", "does not match the linked issue"},
		"missing url":   {`printf '{"description":"x","state":"opened"}'` + "\n", "does not match the linked issue"},
		"other project": {`printf '{"web_url":"https://gitlab.example.com/acme/other/-/issues/69","description":"x"}'` + "\n", "does not match the linked issue"},
	} {
		t.Run(name, func(t *testing.T) {
			repo := installGlab(t, tt.body)
			_, err := NewProvider().ReadIssueSnapshot(t.Context(), executionissue.ExecutionIssueSnapshotRequest{Repo: repo, URL: url})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestGitLabCloseIssueConfirmFailurePaths(t *testing.T) {
	req := port.IssueProviderCloseIssueRequest{IssueURL: "https://gitlab.example.com/acme/repo/-/issues/7", Confirm: true}
	t.Run("bad url", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		res, err := NewProvider().CloseIssue(t.Context(), port.IssueProviderCloseIssueRequest{IssueURL: "https://example.com/x", Confirm: true})
		if err == nil || res.OK || res.Provider != "gitlab" {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
	t.Run("initial state read fails", func(t *testing.T) {
		req := req
		req.Repo = installGlab(t, failScript)
		if res, err := NewProvider().CloseIssue(t.Context(), req); err == nil || res.OK {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
	t.Run("state payload malformed", func(t *testing.T) {
		req := req
		req.Repo = installGlab(t, "printf 'nope'\n")
		if _, err := NewProvider().CloseIssue(t.Context(), req); err == nil || !strings.Contains(err.Error(), "parse glab issue state") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("already closed skips the mutation", func(t *testing.T) {
		req := req
		req.Repo = installGlab(t, `case "$*" in *"--method PUT"*) exit 9 ;; esac
printf '{"state":"Closed"}'
`)
		res, err := NewProvider().CloseIssue(t.Context(), req)
		if err != nil || !res.Closed || !res.AlreadyClosed || res.State != "Closed" {
			t.Fatalf("res=%+v err=%v", res, err)
		}
		if strings.Contains(gapCalls(t, req.Repo), "PUT") {
			t.Fatal("an already closed issue must not be mutated")
		}
	})
	t.Run("close mutation fails", func(t *testing.T) {
		req := req
		req.Repo = installGlab(t, `case "$*" in *"--method PUT"*) echo denied >&2; exit 1 ;; esac
printf '{"state":"opened"}'
`)
		if res, err := NewProvider().CloseIssue(t.Context(), req); err == nil || res.OK || res.Closed {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
	t.Run("readback after close still open", func(t *testing.T) {
		req := req
		req.Repo = installGlab(t, `case "$*" in *"--method PUT"*) printf '{}'; exit 0 ;; esac
printf '{"state":"opened"}'
`)
		res, err := NewProvider().CloseIssue(t.Context(), req)
		if err == nil || !strings.Contains(err.Error(), "close was not verified: state=opened") || res.OK || res.Closed || res.State != "opened" {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
	t.Run("readback after close fails", func(t *testing.T) {
		req := req
		marker := "put.done"
		req.Repo = installGlab(t, `case "$*" in *"--method PUT"*) : > `+marker+`; printf '{}'; exit 0 ;; esac
if [ -f `+marker+` ]; then exit 1; fi
printf '{"state":"opened"}'
`)
		if res, err := NewProvider().CloseIssue(t.Context(), req); err == nil || res.OK {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
}

func TestGitLabCloseMergeRequestConfirmFailurePaths(t *testing.T) {
	req := port.IssueProviderClosePullRequestRequest{ArtifactURL: "https://gitlab.example.com/acme/repo/-/merge_requests/7", Kind: "mr", Confirm: true}
	t.Run("initial state read fails", func(t *testing.T) {
		req := req
		req.Repo = installGlab(t, failScript)
		if res, err := NewProvider().ClosePullRequest(t.Context(), req); err == nil || res.OK {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
	t.Run("state payload malformed", func(t *testing.T) {
		req := req
		req.Repo = installGlab(t, "printf 'nope'\n")
		if _, err := NewProvider().ClosePullRequest(t.Context(), req); err == nil || !strings.Contains(err.Error(), "parse glab merge request state") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("close mutation fails", func(t *testing.T) {
		req := req
		req.Repo = installGlab(t, `case "$*" in *"--method PUT"*) exit 1 ;; esac
printf '{"state":"opened"}'
`)
		if res, err := NewProvider().ClosePullRequest(t.Context(), req); err == nil || res.Closed {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
	t.Run("readback after close still open", func(t *testing.T) {
		req := req
		req.Repo = installGlab(t, `case "$*" in *"--method PUT"*) printf '{}'; exit 0 ;; esac
printf '{"state":"opened"}'
`)
		res, err := NewProvider().ClosePullRequest(t.Context(), req)
		if err == nil || !strings.Contains(err.Error(), "close was not verified: state=opened") || res.OK || res.Closed || res.Kind != "mr" || res.State != "opened" {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
	t.Run("readback after close fails", func(t *testing.T) {
		req := req
		req.Repo = installGlab(t, `case "$*" in *"--method PUT"*) : > put.done; printf '{}'; exit 0 ;; esac
if [ -f put.done ]; then exit 1; fi
printf '{"state":"opened"}'
`)
		if res, err := NewProvider().ClosePullRequest(t.Context(), req); err == nil || res.OK {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
}

func TestGitLabUpdateIssueBodySectionFailurePaths(t *testing.T) {
	base := port.IssueProviderUpdateIssueBodySectionRequest{
		Section: completionmodel.IssueBodySectionDevilsAdvocate, Verdict: "stop", Findings: []string{"f"},
		IssueURL: "https://gitlab.example.com/acme/repo/-/issues/12", Confirm: true,
	}
	t.Run("bad issue url", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		req := base
		req.IssueURL = "https://example.com/x"
		if res, err := NewProvider().UpdateIssueBodySection(t.Context(), req); err == nil || res.OK {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
	t.Run("preview rejects an unrenderable section", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		req := base
		req.Confirm, req.Section = false, "no-such-section"
		if res, err := NewProvider().UpdateIssueBodySection(t.Context(), req); err == nil || res.OK || res.Preview != "" {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
	t.Run("current description read fails", func(t *testing.T) {
		req := base
		req.Repo = installGlab(t, failScript)
		if res, err := NewProvider().UpdateIssueBodySection(t.Context(), req); err == nil || res.OK {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
	t.Run("current description malformed", func(t *testing.T) {
		req := base
		req.Repo = installGlab(t, "printf 'nope'\n")
		if _, err := NewProvider().UpdateIssueBodySection(t.Context(), req); err == nil || !strings.Contains(err.Error(), "parse glab issue description") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("unknown section never writes", func(t *testing.T) {
		req := base
		req.Section = "no-such-section"
		req.Repo = installGlab(t, "printf '{\"description\":\"body\"}'\n")
		if res, err := NewProvider().UpdateIssueBodySection(t.Context(), req); err == nil || res.Updated {
			t.Fatalf("res=%+v err=%v", res, err)
		}
		if strings.Contains(gapCalls(t, req.Repo), "PUT") {
			t.Fatal("an invalid section must not reach the PUT")
		}
	})
	t.Run("PUT fails", func(t *testing.T) {
		req := base
		req.Repo = installGlab(t, `case "$*" in *"--method PUT"*) exit 1 ;; esac
printf '{"description":"body"}'
`)
		if res, err := NewProvider().UpdateIssueBodySection(t.Context(), req); err == nil || res.Updated || res.OK {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
	t.Run("result URL falls back to the requested issue", func(t *testing.T) {
		req := base
		req.Repo = installGlab(t, `printf '{"description":"body"}'`+"\n")
		res, err := NewProvider().UpdateIssueBodySection(t.Context(), req)
		if err != nil || !res.Updated || res.URL != req.IssueURL {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	})
}

func TestGitLabReadPullRequestMergeStateFailurePaths(t *testing.T) {
	t.Run("bad url", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if _, err := NewProvider().ReadPullRequestMergeState(t.Context(), t.TempDir(), "https://gitlab.example/group/repo/-/issues/1"); err == nil {
			t.Fatal("an issue URL is not a merge request")
		}
	})
	t.Run("glab failure", func(t *testing.T) {
		repo := installGlab(t, failScript)
		if _, err := NewProvider().ReadPullRequestMergeState(t.Context(), repo, glMergeTestURL); err == nil {
			t.Fatal("glab failure must surface")
		}
	})
	t.Run("malformed payload", func(t *testing.T) {
		repo := installGlab(t, "printf 'nope'\n")
		if _, err := NewProvider().ReadPullRequestMergeState(t.Context(), repo, glMergeTestURL); err == nil || !strings.Contains(err.Error(), "parse glab merge request merge state") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("closed and merge-commit fallback", func(t *testing.T) {
		repo := installGlab(t, `printf '{"state":"closed","merge_commit_sha":"m1","head_pipeline":{"id":3,"status":"success"}}'`+"\n")
		got, err := NewProvider().ReadPullRequestMergeState(t.Context(), repo, glMergeTestURL)
		if err != nil || got.State != "closed" || got.MergeCommitOID != "m1" || got.Checks != "passing" {
			t.Fatalf("state=%+v err=%v", got, err)
		}
	})
}

func TestGitLabMergePullRequestRejectionsHappenBeforeAnyMutation(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // a glab call would report a missing CLI, not the asserted message
	tests := []struct {
		name string
		req  port.IssueProviderPullRequestMergeRequest
		want string
	}{
		{"bad url", port.IssueProviderPullRequestMergeRequest{URL: "https://gitlab.example/group/repo/-/issues/1", Method: completionmodel.RemoteMergeMethodSquash, HeadOID: "abc"}, "merge request URL"},
		{"blank head", port.IssueProviderPullRequestMergeRequest{URL: glMergeTestURL, Method: completionmodel.RemoteMergeMethodSquash, HeadOID: "  "}, "expected head commit"},
		{"unknown method", port.IssueProviderPullRequestMergeRequest{URL: glMergeTestURL, Method: "", HeadOID: "abc"}, "must be squash, merge, or rebase"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewProvider().MergePullRequest(t.Context(), tt.req); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestGitLabMergePullRequestMergeMethodDisablesSquash(t *testing.T) {
	repo := installGlab(t, `case "$*" in
*"/merge --hostname"*) printf '{}'; exit 0 ;;
esac
printf '{"web_url":"`+glMergeTestURL+`","state":"merged","sha":"abc","merge_commit_sha":"m1"}'
`)
	state, err := NewProvider().MergePullRequest(t.Context(), port.IssueProviderPullRequestMergeRequest{
		Repo: repo, URL: glMergeTestURL, Method: completionmodel.RemoteMergeMethodMerge, HeadOID: " abc ",
	})
	if err != nil || state.State != "merged" || state.MergeCommitOID != "m1" {
		t.Fatalf("state=%+v err=%v", state, err)
	}
	calls := gapCalls(t, repo)
	if !strings.Contains(calls, "-f sha=abc -F squash=false") || strings.Contains(calls, "title=") {
		t.Fatalf("merge method must pass squash=false, a trimmed head, and never touch the title:\n%s", calls)
	}
}

func TestGitLabMergePullRequestMarkReadyFailurePaths(t *testing.T) {
	req := port.IssueProviderPullRequestMergeRequest{URL: glMergeTestURL, Method: completionmodel.RemoteMergeMethodSquash, HeadOID: "abc", MarkReady: true}
	t.Run("title has no draft prefix", func(t *testing.T) {
		req := req
		req.Repo = installGlab(t, `printf '{"title":"fix login","draft":true}'`+"\n")
		_, err := NewProvider().MergePullRequest(t.Context(), req)
		if err == nil || !strings.Contains(err.Error(), "no draft prefix to remove") {
			t.Fatalf("error = %v", err)
		}
		if strings.Contains(gapCalls(t, req.Repo), "PUT") {
			t.Fatal("nothing may be written when the draft marker cannot be cleared")
		}
	})
	t.Run("draft read fails", func(t *testing.T) {
		req := req
		req.Repo = installGlab(t, failScript)
		if _, err := NewProvider().MergePullRequest(t.Context(), req); err == nil {
			t.Fatal("read failure must surface")
		}
	})
	t.Run("draft read malformed", func(t *testing.T) {
		req := req
		req.Repo = installGlab(t, "printf 'nope'\n")
		if _, err := NewProvider().MergePullRequest(t.Context(), req); err == nil || !strings.Contains(err.Error(), "parse glab merge request merge state") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("title update fails before merging", func(t *testing.T) {
		req := req
		req.Repo = installGlab(t, `case "$*" in *"title="*) exit 1 ;; esac
printf '{"title":"Draft: fix"}'
`)
		if _, err := NewProvider().MergePullRequest(t.Context(), req); err == nil {
			t.Fatal("title update failure must surface")
		}
		if strings.Contains(gapCalls(t, req.Repo), "/merge --hostname") {
			t.Fatal("must not merge when the draft marker could not be cleared")
		}
	})
}

func TestGitLabMergePullRequestSurfacesMergeAndReadbackFailures(t *testing.T) {
	req := port.IssueProviderPullRequestMergeRequest{URL: glMergeTestURL, Method: completionmodel.RemoteMergeMethodSquash, HeadOID: "abc"}
	t.Run("merge rejected", func(t *testing.T) {
		req := req
		req.Repo = installGlab(t, "echo 'sha mismatch' >&2\nexit 1\n")
		if _, err := NewProvider().MergePullRequest(t.Context(), req); err == nil {
			t.Fatal("a rejected merge must be an error")
		}
	})
	t.Run("readback fails after merge", func(t *testing.T) {
		req := req
		req.Repo = installGlab(t, `case "$*" in *"/merge --hostname"*) printf '{}'; exit 0 ;; esac
exit 1
`)
		if _, err := NewProvider().MergePullRequest(t.Context(), req); err == nil {
			t.Fatal("an unverifiable merge must be an error")
		}
	})
}

func TestGitLabCreateIssueConfirmRequiresAuthorityAndRejectsMalformedKey(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := NewProvider().CreateIssueContext(t.Context(), port.IssueProviderCreateIssueRequest{Title: "t", Confirm: true})
	if cerr, ok := errors.AsType[*port.IssueProviderCreateError](err); !ok || cerr.Invoked || !strings.Contains(err.Error(), "project authority is required") {
		t.Fatalf("error = %v", err)
	}
	for _, key := range []string{"GitLab.com/acme/repo", "gitlab.com/acme", "gitlab.com/acme/repo?x=1", "user@gitlab.com/acme/repo", "gitlab.com/acme repo"} {
		if _, err := NewProvider().CreateIssueContext(t.Context(), port.IssueProviderCreateIssueRequest{Title: "t", ProjectKey: key}); err == nil || !strings.Contains(err.Error(), "malformed") {
			t.Fatalf("project key %q error = %v", key, err)
		}
	}
}

func TestGitLabCreateIssueConfirmOutcomes(t *testing.T) {
	req := port.IssueProviderCreateIssueRequest{Title: "title", Body: "body", Labels: []string{"bug"}, Assignees: []string{"sample"}, ProjectKey: "gitlab.com/acme/repo", Confirm: true}
	t.Run("returns the canonical URL and passes argv", func(t *testing.T) {
		req := req
		req.Repo = installGlab(t, "printf 'Creating issue\\nhttps://gitlab.com/acme/repo/-/issues/5\\n'\n")
		got, err := NewProvider().CreateIssue(req)
		if err != nil || !got.OK || got.URL != "https://gitlab.com/acme/repo/-/issues/5" {
			t.Fatalf("got=%+v err=%v", got, err)
		}
		want := "issue create --title title --description body --label bug --assignee sample --repo https://gitlab.com/acme/repo"
		if calls := strings.TrimSpace(gapCalls(t, req.Repo)); calls != want {
			t.Fatalf("argv = %q, want %q", calls, want)
		}
	})
	t.Run("success without URL must not be retried", func(t *testing.T) {
		req := req
		req.Repo = installGlab(t, "printf 'done\\n'\n")
		got, err := NewProvider().CreateIssue(req)
		cerr, ok := errors.AsType[*port.IssueProviderCreateError](err)
		if !ok || !cerr.Invoked || got.OK || !strings.Contains(err.Error(), "no canonical URL; do not retry") {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	})
	t.Run("failure with a URL reports unknown outcome and keeps the URL", func(t *testing.T) {
		req := req
		req.Repo = installGlab(t, "printf 'https://gitlab.com/acme/repo/-/issues/6\\n'\necho 'timeout' >&2\nexit 1\n")
		got, err := NewProvider().CreateIssue(req)
		cerr, ok := errors.AsType[*port.IssueProviderCreateError](err)
		if !ok || !cerr.Invoked || got.URL != "https://gitlab.com/acme/repo/-/issues/6" || !strings.Contains(err.Error(), "outcome unknown with a canonical URL") {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	})
	t.Run("failure without a URL reports unknown outcome", func(t *testing.T) {
		req := req
		req.Repo = installGlab(t, "echo 'timeout' >&2\nexit 1\n")
		got, err := NewProvider().CreateIssue(req)
		cerr, ok := errors.AsType[*port.IssueProviderCreateError](err)
		if !ok || !cerr.Invoked || got.OK || got.URL != "" || !strings.Contains(err.Error(), "outcome unknown; do not retry") {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	})
	t.Run("missing glab is reported as not invoked", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		_, err := NewProvider().CreateIssue(req)
		cerr, ok := errors.AsType[*port.IssueProviderCreateError](err)
		if !ok || cerr.Invoked || !strings.Contains(err.Error(), "glab CLI is not installed") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestGitLabGlabAPIReportsMissingCLI(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := runGlabAPIContext(t.Context(), t.TempDir(), "gitlab.example.com", "projects/x"); err == nil || !strings.Contains(err.Error(), "glab CLI is not installed") {
		t.Fatalf("error = %v", err)
	}
	if _, err := runGlabGraphQLContext[map[string]any](t.Context(), t.TempDir(), "", "query { x }", nil); err == nil || !strings.Contains(err.Error(), "glab CLI is not installed") {
		t.Fatalf("graphql error = %v", err)
	}
}

func TestGitLabGraphQLOmitsBlankHostnameAndSortsFields(t *testing.T) {
	repo := installGlab(t, "printf '{\"ok\":true}'\n")
	if _, err := runGlabGraphQLContext[map[string]any](t.Context(), repo, "  ", "query { x }", map[string]string{"zeta": "1", "alpha": "2"}); err != nil {
		t.Fatal(err)
	}
	want := "api graphql -f query=query { x } -f alpha=2 -f zeta=1"
	if got := strings.TrimSpace(gapCalls(t, repo)); got != want {
		t.Fatalf("argv = %q, want %q", got, want)
	}
}

func TestGitLabGraphQLRejectsMalformedAndJoinsErrorMessages(t *testing.T) {
	repo := installGlab(t, "printf 'nope'\n")
	if _, err := runGlabGraphQLContext[map[string]any](t.Context(), repo, "h", "q", nil); err == nil || !strings.Contains(err.Error(), "parse glab graphql response") {
		t.Fatalf("error = %v", err)
	}
	repo = installGlab(t, `printf '{"errors":[{"message":" first "},{"message":"second"}]}'`+"\n")
	if _, err := runGlabGraphQLContext[map[string]any](t.Context(), repo, "h", "q", nil); err == nil || !strings.Contains(err.Error(), "first; second") {
		t.Fatalf("error = %v", err)
	}
}

func TestGitLabChildHierarchyLookupsIgnoreNonHierarchyWidgets(t *testing.T) {
	widgets := []gitlabWorkItemWidget{
		{Type: "LABELS", Children: struct {
			Nodes []gitlabWorkItem `json:"nodes"`
		}{Nodes: []gitlabWorkItem{{ID: "gid://x/1", IID: "1"}}}},
		{Type: "hierarchy", Children: struct {
			Nodes []gitlabWorkItem `json:"nodes"`
		}{Nodes: []gitlabWorkItem{{ID: "gid://x/2", IID: "2", State: "OPEN"}}}},
	}
	if gitlabChildrenContain(widgets, "gid://x/1", "1") {
		t.Fatal("children under a non-hierarchy widget must not count")
	}
	if !gitlabChildrenContain(widgets, "gid://x/2", "") || !gitlabChildrenContain(widgets, "", "2") {
		t.Fatal("hierarchy membership must match by id or by iid, case-insensitively on the widget type")
	}
	if gitlabChildrenContain(widgets, "", "") || gitlabChildrenContain(widgets, "gid://x/9", "9") {
		t.Fatal("blank or unknown identities must never match")
	}
	if got := gitlabChildByIID(widgets, "2"); got.ID != "gid://x/2" || got.State != "OPEN" {
		t.Fatalf("child = %+v", got)
	}
	if got := gitlabChildByIID(widgets, "1"); got.ID != "" {
		t.Fatalf("a child under a non-hierarchy widget leaked: %+v", got)
	}
	if got := gitlabChildByIID(widgets, ""); got.ID != "" {
		t.Fatalf("a blank iid must not match: %+v", got)
	}
}
