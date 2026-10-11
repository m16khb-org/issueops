package github

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	executionissue "issueops/internal/contract/executionissue"
	"issueops/internal/port"
)

const gapParentURL = "https://github.com/acme/repo/issues/12"

func fakeGhRepo(t *testing.T, script string) string {
	t.Helper()
	bin, repo := t.TempDir(), t.TempDir()
	writeFakeGh(t, bin, script)
	t.Setenv("PATH", bin)
	return repo
}

func TestReadIssueSnapshotRejectsBadRequestsAndReadbacks(t *testing.T) {
	const url = "https://github.com/acme/repo/issues/69"
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	cases := []struct {
		name    string
		ctx     context.Context
		url     string
		out     string
		wantErr string
	}{
		{"cancelled context", cancelled, url, `{}`, "context canceled"},
		{"blank url", t.Context(), "  ", `{}`, "issue URL is required"},
		{"gh failure", t.Context(), url, "FAIL", "gh issue snapshot read failed"},
		{"invalid json", t.Context(), url, `not-json`, "parse gh issue snapshot"},
		{"url mismatch", t.Context(), url, `{"url":"https://github.com/acme/repo/issues/70","body":"b","state":"OPEN"}`, "does not match the linked issue"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			script := "#!/bin/sh\nprintf '%s' '" + tc.out + "'\n"
			if tc.out == "FAIL" {
				script = "#!/bin/sh\necho boom >&2\nexit 1\n"
			}
			repo := fakeGhRepo(t, script)
			_, err := NewProvider().ReadIssueSnapshot(tc.ctx, executionissue.ExecutionIssueSnapshotRequest{Repo: repo, URL: tc.url})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want contains %q", err, tc.wantErr)
			}
		})
	}
}

func TestReadIssueSnapshotReturnsStateAndTrimsRequestURL(t *testing.T) {
	repo := fakeGhRepo(t, `#!/bin/sh
printf '%s' '{"url":"https://github.com/acme/repo/issues/69","body":"AC","state":"CLOSED"}'
`)
	got, err := NewProvider().ReadIssueSnapshot(t.Context(), executionissue.ExecutionIssueSnapshotRequest{Repo: repo, URL: "  https://github.com/acme/repo/issues/69\n"})
	if err != nil || got.State != "CLOSED" || got.URL != "https://github.com/acme/repo/issues/69" {
		t.Fatalf("snapshot = %+v, err = %v", got, err)
	}
}

func childAPIScript(issueJSON, subIssuesJSON string) string {
	return `#!/bin/sh
case "$*" in
 "api repos/acme/repo/issues/34") printf '%s' '` + issueJSON + `';;
 "api repos/acme/repo/issues/12/sub_issues?per_page=100") printf '%s' '` + subIssuesJSON + `';;
 *) echo "unexpected: $*" >&2; exit 2;;
esac
`
}

func TestReadChildProjectsVerifiedSnapshot(t *testing.T) {
	repo := fakeGhRepo(t, childAPIScript(
		`{"id":900,"number":34,"html_url":"https://github.com/acme/repo/issues/34","title":"child","body":"b","labels":[{"name":"bug"}],"assignees":[{"login":"octocat"}]}`,
		`[{"id":1,"number":5},{"id":900,"number":34}]`))
	got, err := NewProvider().ReadChild(t.Context(), repo, gapParentURL, "https://github.com/acme/repo/issues/34")
	if err != nil {
		t.Fatal(err)
	}
	if !got.TypeVerified || !got.HierarchyVerified || got.Title != "child" || got.Body != "b" ||
		len(got.Labels) != 1 || got.Labels[0] != "bug" || len(got.Assignees) != 1 || got.Assignees[0] != "octocat" {
		t.Fatalf("snapshot = %+v", got)
	}
}

func TestReadChildFlagsPullRequestAndMissingHierarchy(t *testing.T) {
	repo := fakeGhRepo(t, childAPIScript(
		`{"id":900,"number":34,"html_url":"https://github.com/acme/repo/issues/34","title":"child","pull_request":{"url":"x"}}`,
		`[{"id":1,"number":5}]`))
	got, err := NewProvider().ReadChild(t.Context(), repo, gapParentURL, "https://github.com/acme/repo/issues/34")
	if err != nil {
		t.Fatal(err)
	}
	if got.TypeVerified {
		t.Fatal("a pull request must not verify as an issue child")
	}
	if got.HierarchyVerified {
		t.Fatal("child absent from parent sub-issues must not verify hierarchy")
	}
}

func TestReadChildRejectsIdentityMismatchAndBadInputs(t *testing.T) {
	cases := []struct {
		name, child, issueJSON, wantErr string
	}{
		{"html url differs", "https://github.com/acme/repo/issues/34", `{"id":900,"number":34,"html_url":"https://github.com/acme/repo/issues/35"}`, "identity mismatch"},
		{"number differs", "https://github.com/acme/repo/issues/34", `{"id":900,"number":35,"html_url":"https://github.com/acme/repo/issues/34"}`, "identity mismatch"},
		{"zero id", "https://github.com/acme/repo/issues/34", `{"id":0,"number":34,"html_url":"https://github.com/acme/repo/issues/34"}`, "identity mismatch"},
		{"invalid json", "https://github.com/acme/repo/issues/34", `nope`, "invalid character"},
		{"other project", "https://github.com/other/repo/issues/34", `{}`, "must match linked parent issue project"},
		{"non issue child", "https://github.com/acme/repo/pull/34", `{}`, "numeric github issue"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := fakeGhRepo(t, childAPIScript(tc.issueJSON, `[]`))
			_, err := NewProvider().ReadChild(t.Context(), repo, gapParentURL, tc.child)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want contains %q", err, tc.wantErr)
			}
		})
	}
}

func TestReadChildPropagatesGhFailure(t *testing.T) {
	repo := fakeGhRepo(t, "#!/bin/sh\nexit 1\n")
	if _, err := NewProvider().ReadChild(t.Context(), repo, gapParentURL, "https://github.com/acme/repo/issues/34"); err == nil {
		t.Fatal("expected gh failure to propagate")
	}
}

func TestAttachChildPostsSubIssueIDWithoutReplacingParent(t *testing.T) {
	repo := fakeGhRepo(t, `#!/bin/sh
if [ "$1 $2" = "api repos/acme/repo/issues/34" ]; then printf '%s' '{"id":900,"number":34,"html_url":"https://github.com/acme/repo/issues/34"}'; exit 0; fi
printf '%s\n' "$@" > post.argv
`)
	if err := NewProvider().AttachChild(t.Context(), repo, gapParentURL, "https://github.com/acme/repo/issues/34"); err != nil {
		t.Fatal(err)
	}
	argv, err := os.ReadFile(filepath.Join(repo, "post.argv"))
	want := "api\n-X\nPOST\nrepos/acme/repo/issues/12/sub_issues\n-f\nsub_issue_id=900\n"
	if err != nil || string(argv) != want {
		t.Fatalf("argv = %q, want %q (err %v)", argv, want, err)
	}
	if strings.Contains(string(argv), "replace_parent") {
		t.Fatal("attach must never move an existing parent")
	}
}

func TestAttachChildRejectsMismatchedIdentityWithoutMutation(t *testing.T) {
	repo := fakeGhRepo(t, `#!/bin/sh
if [ "$1 $2" = "api repos/acme/repo/issues/34" ]; then printf '%s' '{"id":900,"number":34,"html_url":"https://github.com/acme/repo/issues/99"}'; exit 0; fi
echo mutated >> mutations
`)
	err := NewProvider().AttachChild(t.Context(), repo, gapParentURL, "https://github.com/acme/repo/issues/34")
	if err == nil || !strings.Contains(err.Error(), "attach identity mismatch") {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(repo, "mutations")); !os.IsNotExist(statErr) {
		t.Fatal("mutation ran despite identity mismatch")
	}
}

func TestAttachChildRejectsInvalidURLsAndSurfacesPostFailure(t *testing.T) {
	repo := fakeGhRepo(t, `#!/bin/sh
if [ "$1 $2" = "api repos/acme/repo/issues/34" ]; then printf '%s' '{"id":900,"number":34,"html_url":"https://github.com/acme/repo/issues/34"}'; exit 0; fi
echo already has parent >&2
exit 1
`)
	p := NewProvider()
	if err := p.AttachChild(t.Context(), repo, gapParentURL, "https://github.com/other/repo/issues/34"); err == nil {
		t.Fatal("cross-project child must be rejected")
	}
	if err := p.AttachChild(t.Context(), repo, gapParentURL, "https://github.com/acme/repo/issues/34"); err == nil {
		t.Fatal("POST failure must surface")
	}
}

func TestFindIssueCreateCandidatesRejectsInvalidRequests(t *testing.T) {
	repo := fakeGhRepo(t, "#!/bin/sh\necho called >> calls\nexit 0\n")
	for name, req := range map[string]port.IssueProviderFindIssueCreateCandidatesRequest{
		"no authority":        {Repo: repo, Marker: "m"},
		"malformed authority": {Repo: repo, ProjectAuthority: "GitHub.com/Acme/Repo", Marker: "m"},
		"blank marker":        {Repo: repo, ProjectAuthority: "github.com/acme/repo", Marker: "  "},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewProvider().FindIssueCreateCandidates(t.Context(), req); err == nil || !strings.Contains(err.Error(), "invalid GitHub issue create reconciliation request") {
				t.Fatalf("err = %v", err)
			}
		})
	}
	if _, err := os.Stat(filepath.Join(repo, "calls")); !os.IsNotExist(err) {
		t.Fatal("gh must not run for invalid requests")
	}
}

func TestFindIssueCreateCandidatesFiltersByMarkerAndUsesExactArgv(t *testing.T) {
	repo := fakeGhRepo(t, `#!/bin/sh
printf '%s\n' "$@" > argv
printf '%s' '[{"url":"https://github.com/acme/repo/issues/1","title":"hit","body":"x issueops-marker-1 y"},{"url":"https://github.com/acme/repo/issues/2","title":"fuzzy","body":"unrelated"}]'
`)
	got, err := NewProvider().FindIssueCreateCandidates(t.Context(), port.IssueProviderFindIssueCreateCandidatesRequest{Repo: repo, ProjectAuthority: " github.com/acme/repo ", Marker: "issueops-marker-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Candidates) != 1 || got.Candidates[0].URL != "https://github.com/acme/repo/issues/1" || got.Candidates[0].Title != "hit" {
		t.Fatalf("candidates = %+v", got.Candidates)
	}
	if got.Truncated {
		t.Fatal("two rows must not be reported as truncated")
	}
	argv, _ := os.ReadFile(filepath.Join(repo, "argv"))
	want := "issue\nlist\n--repo\ngithub.com/acme/repo\n--state\nall\n--limit\n100\n--search\nissueops-marker-1\n--json\nurl,title,body\n"
	if string(argv) != want {
		t.Fatalf("argv = %q, want %q", argv, want)
	}
}

func TestFindIssueCreateCandidatesReportsTruncationAtSearchLimit(t *testing.T) {
	repo := fakeGhRepo(t, `#!/bin/sh
i=0; printf '['
while [ $i -lt 100 ]; do [ $i -gt 0 ] && printf ','; printf '{"url":"u%s","title":"t","body":"MARK"}' $i; i=$((i+1)); done
printf ']'
`)
	got, err := NewProvider().FindIssueCreateCandidates(t.Context(), port.IssueProviderFindIssueCreateCandidatesRequest{Repo: repo, ProjectAuthority: "github.com/acme/repo", Marker: "MARK"})
	if err != nil || !got.Truncated || len(got.Candidates) != 100 {
		t.Fatalf("truncated=%v candidates=%d err=%v", got.Truncated, len(got.Candidates), err)
	}
}

func TestFindChildCreateCandidatesUsesIssueSearchSemantics(t *testing.T) {
	repo := fakeGhRepo(t, `#!/bin/sh
printf '%s' '[{"url":"u1","title":"t","body":"has M"},{"url":"u2","title":"t","body":"no"}]'
`)
	got, err := NewProvider().FindChildCreateCandidates(t.Context(), port.IssueProviderFindIssueCreateCandidatesRequest{Repo: repo, ProjectAuthority: "github.com/acme/repo", Marker: "M"})
	if err != nil || len(got.Candidates) != 1 || got.Candidates[0].URL != "u1" {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestFindIssueCreateCandidatesWrapsGhAndParseFailures(t *testing.T) {
	req := func(repo string) port.IssueProviderFindIssueCreateCandidatesRequest {
		return port.IssueProviderFindIssueCreateCandidatesRequest{Repo: repo, ProjectAuthority: "github.com/acme/repo", Marker: "M"}
	}
	repo := fakeGhRepo(t, "#!/bin/sh\nexit 1\n")
	if _, err := NewProvider().FindIssueCreateCandidates(t.Context(), req(repo)); err == nil || !strings.Contains(err.Error(), "gh issue reconciliation failed") {
		t.Fatalf("err = %v", err)
	}
	repo = fakeGhRepo(t, "#!/bin/sh\nprintf '{not json'\n")
	if _, err := NewProvider().FindIssueCreateCandidates(t.Context(), req(repo)); err == nil || !strings.Contains(err.Error(), "parse gh issue reconciliation response") {
		t.Fatalf("err = %v", err)
	}
}

func TestCreateIssueConfirmRequiresProjectAuthorityBeforeInvoking(t *testing.T) {
	repo := fakeGhRepo(t, "#!/bin/sh\necho called >> calls\n")
	_, err := NewProvider().CreateIssue(port.IssueProviderCreateIssueRequest{Repo: repo, Title: "t", Confirm: true})
	typed, ok := errors.AsType[*port.IssueProviderCreateError](err)
	if !ok || typed.Invoked {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(repo, "calls")); !os.IsNotExist(statErr) {
		t.Fatal("gh ran without project authority")
	}
}

func TestCreateIssueRejectsMalformedProjectKey(t *testing.T) {
	_, err := NewProvider().CreateIssue(port.IssueProviderCreateIssueRequest{Title: "t", ProjectKey: "GitHub.com/Acme/Repo"})
	if err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("err = %v", err)
	}
}

func TestCreateIssueConfirmReturnsNumberFromCanonicalURL(t *testing.T) {
	repo := fakeGhRepo(t, `#!/bin/sh
printf '%s\n' "$@" > argv
printf 'https://github.com/acme/repo/issues/77\n'
`)
	got, err := NewProvider().CreateIssue(port.IssueProviderCreateIssueRequest{Repo: repo, ProjectKey: "github.com/acme/repo", Title: " T ", Body: " B ", Labels: []string{"bug"}, Assignees: []string{"octocat"}, Confirm: true})
	if err != nil || !got.OK || got.Number != "77" || got.URL != "https://github.com/acme/repo/issues/77" {
		t.Fatalf("got %+v err %v", got, err)
	}
	argv, _ := os.ReadFile(filepath.Join(repo, "argv"))
	want := "issue\ncreate\n--title\nT\n--body\nB\n--label\nbug\n--assignee\noctocat\n--repo\ngithub.com/acme/repo\n"
	if string(argv) != want {
		t.Fatalf("argv = %q, want %q", argv, want)
	}
}

func TestCreateIssueNonCanonicalURLNeedsReconciliationNotRetry(t *testing.T) {
	repo := fakeGhRepo(t, "#!/bin/sh\nprintf 'created something\\n'\n")
	got, err := NewProvider().CreateIssue(port.IssueProviderCreateIssueRequest{Repo: repo, ProjectKey: "github.com/acme/repo", Title: "t", Confirm: true})
	typed, ok := errors.AsType[*port.IssueProviderCreateError](err)
	if got.OK || !ok || !typed.Invoked || !strings.Contains(err.Error(), "needs reconciliation") {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestCreateIssueFailureAfterStartKeepsCanonicalURLAndNumber(t *testing.T) {
	repo := fakeGhRepo(t, "#!/bin/sh\nprintf 'https://github.com/acme/repo/issues/78\\n'\necho 'network lost' >&2\nexit 1\n")
	got, err := NewProvider().CreateIssue(port.IssueProviderCreateIssueRequest{Repo: repo, ProjectKey: "github.com/acme/repo", Title: "t", Confirm: true})
	typed, ok := errors.AsType[*port.IssueProviderCreateError](err)
	if got.OK || !ok || !typed.Invoked || got.URL != "https://github.com/acme/repo/issues/78" || got.Number != "78" || !strings.Contains(err.Error(), "do not retry") {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestCreatePullRequestFailureAfterStartIsNeverRetriedAndCarriesNoURLWhenNonCanonical(t *testing.T) {
	prReq := func(repo string) port.IssueProviderCreatePullRequestRequest {
		return port.IssueProviderCreatePullRequestRequest{Repo: repo, ProjectKey: "github.com/acme/repo", Title: "PR", HeadBranch: "feat/x", BaseBranch: "main", Confirm: true}
	}
	repo := fakeGhRepo(t, "#!/bin/sh\necho x >> calls\nprintf 'garbage\\n'\nexit 1\n")
	_, err := NewProvider().CreatePullRequest(prReq(repo))
	typed, ok := errors.AsType[*port.IssueProviderCreateError](err)
	if !ok || !typed.Invoked || !strings.Contains(err.Error(), "outcome unknown; do not retry") {
		t.Fatalf("err = %v", err)
	}
	calls, _ := os.ReadFile(filepath.Join(repo, "calls"))
	if strings.Count(string(calls), "x") != 1 {
		t.Fatalf("create retried: %q", calls)
	}

	repo = fakeGhRepo(t, "#!/bin/sh\nprintf 'https://github.com/acme/repo/pull/9\\n'\nexit 1\n")
	res, err := NewProvider().CreatePullRequest(prReq(repo))
	if err == nil || !strings.Contains(err.Error(), "canonical URL returned separately") || res.OK {
		t.Fatalf("res %+v err %v", res, err)
	}
}

func TestCreatePullRequestMissingGhIsNotInvoked(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := NewProvider().CreatePullRequest(port.IssueProviderCreatePullRequestRequest{Repo: t.TempDir(), ProjectKey: "github.com/acme/repo", Title: "PR", HeadBranch: "feat/x", BaseBranch: "main", Confirm: true})
	typed, ok := errors.AsType[*port.IssueProviderCreateError](err)
	if !ok || typed.Invoked {
		t.Fatalf("err = %v", err)
	}
}

func TestReadPullRequestMergeStateRejectsNonCanonicalURLAndGhFailure(t *testing.T) {
	repo := fakeGhRepo(t, "#!/bin/sh\nexit 1\n")
	p := NewProvider()
	for _, u := range []string{"", "http://github.com/a/b/pull/1", "https://github.com/a/b/issues/1", "https://github.com/a/b/pull/0", "https://github.com/a/b/pull/1?x=1"} {
		if _, err := p.ReadPullRequestMergeState(t.Context(), repo, u); err == nil || !strings.Contains(err.Error(), "canonical") {
			t.Fatalf("%q: err = %v", u, err)
		}
	}
	if _, err := p.ReadPullRequestMergeState(t.Context(), repo, "https://github.com/a/b/pull/1"); err == nil || !strings.Contains(err.Error(), "gh pr view failed") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseGhMergeStateClassifiesChecksAndBlocking(t *testing.T) {
	cases := []struct {
		name, json, checks, blocked string
		failing, pending            []string
		conflict                    bool
	}{
		{"failing run and legacy status", `{"state":"OPEN","mergeable":"CONFLICTING","mergeStateStatus":"BLOCKED","statusCheckRollup":[{"__typename":"CheckRun","name":"build","status":"COMPLETED","conclusion":"FAILURE"},{"__typename":"StatusContext","context":"ci/legacy","state":"ERROR"},{"__typename":"CheckRun","name":"ok","status":"COMPLETED","conclusion":"SUCCESS"}]}`, "failing", "", []string{"build", "ci/legacy"}, nil, true},
		{"pending wins over passing", `{"state":"OPEN","statusCheckRollup":[{"__typename":"CheckRun","name":"slow","status":"IN_PROGRESS"},{"__typename":"StatusContext","context":"exp","state":"EXPECTED"},{"__typename":"StatusContext","context":"fine","state":"SUCCESS"}]}`, "pending", "", nil, []string{"slow", "exp"}, false},
		{"all passing incl neutral and skipped", `{"state":"OPEN","statusCheckRollup":[{"name":"a","status":"completed","conclusion":"neutral"},{"name":"b","status":"COMPLETED","conclusion":"SKIPPED"}]}`, "passing", "", nil, nil, false},
		{"no checks but blocked by review", `{"state":"OPEN","mergeStateStatus":"BLOCKED"}`, "none", "BLOCKED (required review or branch protection)", nil, nil, false},
		{"blocked explained by failing checks", `{"state":"OPEN","mergeStateStatus":"blocked","statusCheckRollup":[{"name":"x","status":"COMPLETED","conclusion":"CANCELLED"}]}`, "failing", "", []string{"x"}, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseGhMergeState([]byte(tc.json))
			if err != nil {
				t.Fatal(err)
			}
			if got.Checks != tc.checks || got.Blocked != tc.blocked || got.Conflict != tc.conflict ||
				strings.Join(got.Failing, ",") != strings.Join(tc.failing, ",") || strings.Join(got.Pending, ",") != strings.Join(tc.pending, ",") {
				t.Fatalf("state = %+v", got)
			}
		})
	}
	got, err := parseGhMergeState([]byte(`{"state":"MERGED","mergeCommit":{"oid":"abc"}}`))
	if err != nil || got.State != "merged" || got.MergeCommitOID != "abc" {
		t.Fatalf("merged state = %+v err %v", got, err)
	}
	if _, err := parseGhMergeState([]byte(`oops`)); err == nil {
		t.Fatal("invalid JSON must fail")
	}
}
