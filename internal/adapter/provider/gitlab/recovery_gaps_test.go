package gitlab

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/port"
)

const (
	gapParentURL = "https://gitlab.example.com/acme/repo/-/issues/12"
	gapChildURL  = "https://gitlab.example.com/acme/repo/-/work_items/34"
)

// gapRecoveryItem renders a childRecovery lookup response whose single node is
// the child, with the supplied widgets JSON array body.
func gapRecoveryItem(itemType, widgets string) string {
	return `{"data":{"project":{"workItems":{"nodes":[{"id":"gid://gitlab/WorkItem/34","iid":"34","webUrl":"` + gapChildURL +
		`","title":"child","description":"body","workItemType":{"name":"` + itemType + `"},"widgets":[` + widgets + `]}],"pageInfo":{"hasNextPage":false}}}}}`
}

func gapHierarchy(parentURL string) string {
	if parentURL == "" {
		return `{"type":"HIERARCHY","hasParent":false,"parent":null}`
	}
	return `{"type":"HIERARCHY","hasParent":true,"parent":{"id":"gid://gitlab/WorkItem/12","webUrl":"` + parentURL + `"}}`
}

// installRecoveryGlab serves lookupJSON for the child lookup and logs every call.
func installRecoveryGlab(t *testing.T, lookupJSON, attachJSON, parentJSON string) (repo string) {
	t.Helper()
	skipOnWindows(t)
	binDir := t.TempDir()
	repo = t.TempDir()
	writeFakeGlab(t, binDir, `#!/bin/sh
printf '%s\n' "$*" >> glab.calls
case "$*" in
  *childRecoveryAttach*) printf '%s' '`+attachJSON+`'; exit 0 ;;
  *childRecovery*) printf '%s' '`+lookupJSON+`'; exit 0 ;;
  *parentIid*) printf '%s' '`+parentJSON+`'; exit 0 ;;
esac
echo "unexpected glab call: $*" >&2
exit 2
`)
	t.Setenv("PATH", binDir)
	return repo
}

func gapCalls(t *testing.T, repo string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repo, "glab.calls"))
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatal(err)
	}
	return string(data)
}

func TestGitLabReadChildReportsLabelsAssigneesAndVerifiedHierarchy(t *testing.T) {
	widgets := gapHierarchy(gapParentURL) +
		`,{"type":"LABELS","labels":{"nodes":[{"title":"bug"},{"title":"ops"}]}},{"type":"ASSIGNEES","assignees":{"nodes":[{"username":"sample"}]}}`
	repo := installRecoveryGlab(t, gapRecoveryItem("Task", widgets), `{}`, `{}`)
	got, err := NewProvider().ReadChild(context.Background(), repo, gapParentURL, gapChildURL)
	if err != nil {
		t.Fatal(err)
	}
	if !got.TypeVerified || !got.HierarchyVerified || got.URL != gapChildURL || got.Title != "child" || got.Body != "body" {
		t.Fatalf("snapshot = %+v", got)
	}
	if strings.Join(got.Labels, ",") != "bug,ops" || strings.Join(got.Assignees, ",") != "sample" {
		t.Fatalf("labels/assignees = %v / %v", got.Labels, got.Assignees)
	}
	calls := gapCalls(t, repo)
	for _, want := range []string{"--hostname gitlab.example.com", "childIid=34", "projectPath=acme/repo"} {
		if !strings.Contains(calls, want) {
			t.Fatalf("lookup call missing %q: %s", want, calls)
		}
	}
}

func TestGitLabReadChildDoesNotVerifyANonTaskOrUnattachedChild(t *testing.T) {
	repo := installRecoveryGlab(t, gapRecoveryItem("Issue", gapHierarchy("")), `{}`, `{}`)
	got, err := NewProvider().ReadChild(context.Background(), repo, gapParentURL, gapChildURL)
	if err != nil {
		t.Fatal(err)
	}
	if got.TypeVerified || got.HierarchyVerified {
		t.Fatalf("a non-Task child without a parent must verify neither type nor hierarchy: %+v", got)
	}
}

func TestGitLabReadChildRejectsAmbiguousHierarchyAndIdentity(t *testing.T) {
	otherParent := "https://gitlab.example.com/acme/repo/-/issues/99"
	tests := []struct {
		name, lookup, wantErr string
	}{
		{"different parent", gapRecoveryItem("Task", gapHierarchy(otherParent)), "different or unobservable parent"},
		{"parent present without hasParent flag", gapRecoveryItem("Task", `{"type":"HIERARCHY","hasParent":false,"parent":{"id":"x","webUrl":"`+otherParent+`"}}`), "child has a different parent"},
		{"unobservable parent", gapRecoveryItem("Task", `{"type":"HIERARCHY","hasParent":true,"parent":null}`), "different or unobservable parent"},
		{"missing hierarchy widget", gapRecoveryItem("Task", `{"type":"LABELS","labels":{"nodes":[]}}`), "hierarchy widget missing"},
		{"no node", `{"data":{"project":{"workItems":{"nodes":[],"pageInfo":{"hasNextPage":false}}}}}`, "identity mismatch"},
		{"more pages", strings.Replace(gapRecoveryItem("Task", gapHierarchy("")), `"hasNextPage":false`, `"hasNextPage":true`, 1), "identity mismatch"},
		{"other iid", strings.Replace(gapRecoveryItem("Task", gapHierarchy("")), `"iid":"34"`, `"iid":"35"`, 1), "identity mismatch"},
		{"other url", strings.Replace(gapRecoveryItem("Task", gapHierarchy("")), gapChildURL, gapChildURL+"0", 1), "identity mismatch"},
		{"empty id", strings.Replace(gapRecoveryItem("Task", gapHierarchy("")), `"id":"gid://gitlab/WorkItem/34"`, `"id":""`, 1), "identity mismatch"},
		{"graphql error", `{"errors":[{"message":"denied"}]}`, "denied"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := installRecoveryGlab(t, tt.lookup, `{}`, `{}`)
			_, err := NewProvider().ReadChild(context.Background(), repo, gapParentURL, gapChildURL)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestGitLabReadChildRejectsMismatchedURLsBeforeAnyGlabCall(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // any glab call would report a missing CLI instead
	tests := []struct{ name, parent, child, wantErr string }{
		{"other project", gapParentURL, "https://gitlab.example.com/acme/other/-/work_items/34", "match linked parent"},
		{"child is a plain issue url", gapParentURL, "https://gitlab.example.com/acme/repo/-/issues/34", "work item URL"},
		{"non numeric child", gapParentURL, "https://gitlab.example.com/acme/repo/-/work_items/abc", "numeric"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewProvider().ReadChild(context.Background(), t.TempDir(), tt.parent, tt.child)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestGitLabAttachChildIsNoopWhenHierarchyAlreadyVerified(t *testing.T) {
	repo := installRecoveryGlab(t, gapRecoveryItem("Task", gapHierarchy(gapParentURL)), `{}`, `{}`)
	if err := NewProvider().AttachChild(context.Background(), repo, gapParentURL, gapChildURL); err != nil {
		t.Fatal(err)
	}
	calls := gapCalls(t, repo)
	if strings.Contains(calls, "childRecoveryAttach") || strings.Contains(calls, "parentIid") {
		t.Fatalf("an attached child must not be re-attached:\n%s", calls)
	}
}

func TestGitLabAttachChildAddsUnattachedChildToParent(t *testing.T) {
	repo := installRecoveryGlab(t, gapRecoveryItem("Task", gapHierarchy("")),
		`{"data":{"workItemHierarchyAddChildrenItems":{"errors":[]}}}`,
		`{"data":{"project":{"issue":{"id":"gid://gitlab/WorkItem/12"}}}}`)
	if err := NewProvider().AttachChild(context.Background(), repo, gapParentURL, gapChildURL); err != nil {
		t.Fatal(err)
	}
	calls := gapCalls(t, repo)
	for _, want := range []string{"childRecoveryAttach", "parentId=gid://gitlab/WorkItem/12", "childId=gid://gitlab/WorkItem/34", "parentIid=12"} {
		if !strings.Contains(calls, want) {
			t.Fatalf("attach calls missing %q:\n%s", want, calls)
		}
	}
}

func TestGitLabAttachChildSurfacesParentAndMutationFailures(t *testing.T) {
	unattached := gapRecoveryItem("Task", gapHierarchy(""))
	parentOK := `{"data":{"project":{"issue":{"id":"gid://gitlab/WorkItem/12"}}}}`
	tests := []struct{ name, attach, parent, wantErr string }{
		{"mutation errors", `{"data":{"workItemHierarchyAddChildrenItems":{"errors":["already linked","type mismatch"]}}}`, parentOK, "already linked, type mismatch"},
		{"parent id missing", `{}`, `{"data":{"project":{"issue":{"id":""}}}}`, "parent lookup identity missing"},
		{"graphql envelope error", `{"errors":[{"message":"no permission"}]}`, parentOK, "no permission"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := installRecoveryGlab(t, unattached, tt.attach, tt.parent)
			err := NewProvider().AttachChild(context.Background(), repo, gapParentURL, gapChildURL)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestGitLabAttachChildPropagatesReadFailureWithoutMutating(t *testing.T) {
	repo := installRecoveryGlab(t, gapRecoveryItem("Task", gapHierarchy("https://gitlab.example.com/acme/repo/-/issues/99")), `{}`, `{}`)
	err := NewProvider().AttachChild(context.Background(), repo, gapParentURL, gapChildURL)
	if err == nil || !strings.Contains(err.Error(), "different or unobservable parent") {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(gapCalls(t, repo), "childRecoveryAttach") {
		t.Fatal("a child under another parent must never be re-parented")
	}
}

func TestGitLabFindChildCreateCandidatesFiltersByMarkerAndFlagsTruncation(t *testing.T) {
	skipOnWindows(t)
	binDir, repo := t.TempDir(), t.TempDir()
	writeFakeGlab(t, binDir, `#!/bin/sh
printf '%s\n' "$*" >> glab.calls
printf '%s' '{"data":{"project":{"workItems":{"nodes":[{"webUrl":"https://gitlab.example.com/acme/repo/-/work_items/1","title":"hit","description":"x MARK-1 y"},{"webUrl":"https://gitlab.example.com/acme/repo/-/work_items/2","title":"fuzzy","description":"unrelated"}],"pageInfo":{"hasNextPage":true}}}}}'
`)
	t.Setenv("PATH", binDir)
	got, err := NewProvider().FindChildCreateCandidates(context.Background(), port.IssueProviderFindIssueCreateCandidatesRequest{
		Repo: repo, ProjectAuthority: "gitlab.example.com/acme/repo", Marker: "MARK-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Candidates) != 1 || got.Candidates[0].Title != "hit" || got.Candidates[0].Body != "x MARK-1 y" {
		t.Fatalf("only the marker-bearing child may be a candidate: %+v", got.Candidates)
	}
	if !got.Truncated {
		t.Fatal("hasNextPage must flag the search as truncated")
	}
	calls := gapCalls(t, repo)
	for _, want := range []string{"--hostname gitlab.example.com", "projectPath=acme/repo", "marker=MARK-1"} {
		if !strings.Contains(calls, want) {
			t.Fatalf("search call missing %q: %s", want, calls)
		}
	}
}

func TestGitLabFindChildCreateCandidatesRejectsInvalidRequestsAndGraphQLErrors(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, req := range []port.IssueProviderFindIssueCreateCandidatesRequest{
		{ProjectAuthority: "no-slash", Marker: "m"},
		{ProjectAuthority: "gitlab.example.com/acme/repo", Marker: ""},
	} {
		if _, err := NewProvider().FindChildCreateCandidates(context.Background(), req); err == nil || !strings.Contains(err.Error(), "invalid child recovery search") {
			t.Fatalf("req %+v error = %v", req, err)
		}
	}
	skipOnWindows(t)
	binDir := t.TempDir()
	writeFakeGlab(t, binDir, "#!/bin/sh\nprintf '%s' '{\"errors\":[{\"message\":\"rate limited\"}]}'\n")
	t.Setenv("PATH", binDir)
	_, err := NewProvider().FindChildCreateCandidates(context.Background(), port.IssueProviderFindIssueCreateCandidatesRequest{
		Repo: t.TempDir(), ProjectAuthority: "gitlab.example.com/acme/repo", Marker: "m",
	})
	if err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("error = %v", err)
	}
}

func TestGitLabFindIssueCreateCandidatesFiltersByMarkerAndOmitsDefaultHostname(t *testing.T) {
	skipOnWindows(t)
	binDir, repo := t.TempDir(), t.TempDir()
	writeFakeGlab(t, binDir, `#!/bin/sh
printf '%s\n' "$*" >> glab.calls
printf '%s' '[{"web_url":"https://gitlab.com/acme/repo/-/issues/1","title":"hit","description":"has M-9"},{"web_url":"https://gitlab.com/acme/repo/-/issues/2","title":"miss","description":"none"}]'
`)
	t.Setenv("PATH", binDir)
	got, err := NewProvider().FindIssueCreateCandidates(context.Background(), port.IssueProviderFindIssueCreateCandidatesRequest{
		Repo: repo, ProjectAuthority: " gitlab.com/acme/repo ", Marker: "M-9",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Candidates) != 1 || got.Candidates[0].URL != "https://gitlab.com/acme/repo/-/issues/1" || got.Truncated {
		t.Fatalf("result = %+v", got)
	}
	calls := gapCalls(t, repo)
	if strings.Contains(calls, "--hostname") || !strings.Contains(calls, "projects/acme%2Frepo/issues?scope=all&per_page=100&in=description&search=M-9") {
		t.Fatalf("gitlab.com must use the default host and an escaped search endpoint: %s", calls)
	}
}

func TestGitLabFindIssueCreateCandidatesPassesSelfHostedHostAndFlagsFullPage(t *testing.T) {
	skipOnWindows(t)
	binDir, repo := t.TempDir(), t.TempDir()
	writeFakeGlab(t, binDir, `#!/bin/sh
printf '%s\n' "$*" >> glab.calls
i=0
printf '['
while [ $i -lt 100 ]; do
  [ $i -gt 0 ] && printf ','
  printf '{"web_url":"https://git.corp.example/acme/repo/-/issues/%s","title":"t","description":"M-9"}' "$i"
  i=$((i+1))
done
printf ']'
`)
	t.Setenv("PATH", binDir)
	got, err := NewProvider().FindIssueCreateCandidates(context.Background(), port.IssueProviderFindIssueCreateCandidatesRequest{
		Repo: repo, ProjectAuthority: "git.corp.example/acme/repo", Marker: "M-9",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Truncated || len(got.Candidates) != 100 {
		t.Fatalf("a full page must be flagged truncated: truncated=%v candidates=%d", got.Truncated, len(got.Candidates))
	}
	if !strings.Contains(gapCalls(t, repo), "--hostname git.corp.example") {
		t.Fatalf("self-hosted authority must select its hostname: %s", gapCalls(t, repo))
	}
}

func TestGitLabFindIssueCreateCandidatesRejectsBadRequestsAndResponses(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, req := range []port.IssueProviderFindIssueCreateCandidatesRequest{
		{ProjectAuthority: "gitlab.com", Marker: "m"},
		{ProjectAuthority: "/acme/repo", Marker: "m"},
		{ProjectAuthority: "gitlab.com/acme/repo", Marker: "  "},
	} {
		if _, err := NewProvider().FindIssueCreateCandidates(context.Background(), req); err == nil || !strings.Contains(err.Error(), "invalid GitLab issue create reconciliation request") {
			t.Fatalf("req %+v error = %v", req, err)
		}
	}
	skipOnWindows(t)
	for name, script := range map[string]string{
		"malformed json": "#!/bin/sh\nprintf 'not json'\n",
		"glab failure":   "#!/bin/sh\necho boom >&2\nexit 1\n",
	} {
		t.Run(name, func(t *testing.T) {
			binDir := t.TempDir()
			writeFakeGlab(t, binDir, script)
			t.Setenv("PATH", binDir)
			_, err := NewProvider().FindIssueCreateCandidates(context.Background(), port.IssueProviderFindIssueCreateCandidatesRequest{
				Repo: t.TempDir(), ProjectAuthority: "gitlab.com/acme/repo", Marker: "m",
			})
			if err == nil || !strings.Contains(err.Error(), "glab issue reconciliation") && !strings.Contains(err.Error(), "parse glab issue reconciliation") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestGitLabFindChildCreateCandidatesFlagsTruncationAtFullPageWithoutNextPage(t *testing.T) {
	skipOnWindows(t)
	for _, tt := range []struct {
		nodes int
		want  bool
	}{{99, false}, {100, true}} {
		t.Run(fmt.Sprintf("%d nodes", tt.nodes), func(t *testing.T) {
			binDir, repo := t.TempDir(), t.TempDir()
			nodes := strings.TrimSuffix(strings.Repeat(`{"webUrl":"https://gitlab.example.com/acme/repo/-/work_items/1","title":"t","description":"MARK-1"},`, tt.nodes), ",")
			writeFakeGlab(t, binDir, "#!/bin/sh\nprintf '%s' '{\"data\":{\"project\":{\"workItems\":{\"nodes\":["+nodes+"],\"pageInfo\":{\"hasNextPage\":false}}}}}'\n")
			t.Setenv("PATH", binDir)
			got, err := NewProvider().FindChildCreateCandidates(context.Background(), port.IssueProviderFindIssueCreateCandidatesRequest{
				Repo: repo, ProjectAuthority: "gitlab.example.com/acme/repo", Marker: "MARK-1",
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Candidates) != tt.nodes || got.Truncated != tt.want {
				t.Fatalf("candidates=%d truncated=%v, want %d/%v", len(got.Candidates), got.Truncated, tt.nodes, tt.want)
			}
		})
	}
}
