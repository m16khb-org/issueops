package gitlab

import (
	"errors"
	"maps"
	"strings"
	"testing"

	"issueops/internal/port"
)

const gapChildWebURL = "https://gitlab.com/acme/repo/-/work_items/34"

// childFlowResponses are the happy-path GraphQL answers keyed by the operation
// name that appears in the query. A test overrides one key to break one step.
func childFlowResponses() map[string]string {
	widgets := `[{"type":"ASSIGNEES","assignees":{"nodes":[{"username":"sample"}]}},{"type":"HIERARCHY"},{"type":"LABELS","labels":{"nodes":[{"title":"bug"}]}}]`
	return map[string]string{
		"taskType":                          `{"data":{"namespace":{"workItemTypes":{"nodes":[{"id":"gid://gitlab/WorkItems::Type/2","name":"Task"}]}}}}`,
		"labelLookup":                       `{"data":{"project":{"labels":{"nodes":[{"id":"gid://gitlab/ProjectLabel/7","title":"bug"}]}}}}`,
		"userLookup":                        `{"data":{"user":{"id":"gid://gitlab/User/9","username":"sample"}}}`,
		"workItemCreate":                    `{"data":{"workItemCreate":{"workItem":{"id":"gid://gitlab/WorkItem/34","iid":"34","webUrl":"` + gapChildWebURL + `","widgets":` + widgets + `},"errors":[]}}}`,
		"parentIid":                         `{"data":{"project":{"issue":{"id":"gid://gitlab/WorkItem/12"}}}}`,
		"workItemHierarchyAddChildrenItems": `{"data":{"workItemHierarchyAddChildrenItems":{"errors":[]}}}`,
		"children":                          `{"data":{"workItem":{"widgets":[{"type":"HIERARCHY","children":{"nodes":[{"id":"gid://gitlab/WorkItem/34","iid":"34","webUrl":"` + gapChildWebURL + `","state":"OPEN"}]}}]}}}`,
		"childVerify":                       `{"data":{"workItem":{"iid":"34","webUrl":"` + gapChildWebURL + `","widgets":` + widgets + `}}}`,
		"workItemUpdate":                    `{"data":{"workItemUpdate":{"workItem":{"id":"gid://gitlab/WorkItem/34","state":"CLOSED"},"errors":[]}}}`,
		"childCloseVerify":                  `{"data":{"workItem":{"id":"gid://gitlab/WorkItem/34","iid":"34","webUrl":"` + gapChildWebURL + `","state":"CLOSED"}}}`,
	}
}

// installChildFlowGlab serves responses by operation name; the value FAIL makes
// that operation exit non-zero. Order matters: names that are substrings of
// other queries come last.
func installChildFlowGlab(t *testing.T, overrides map[string]string) string {
	t.Helper()
	responses := childFlowResponses()
	maps.Copy(responses, overrides)
	var script strings.Builder
	script.WriteString("case \"$*\" in\n")
	for _, name := range []string{"taskType", "labelLookup", "userLookup", "workItemCreate", "parentIid", "workItemHierarchyAddChildrenItems", "childCloseVerify", "workItemUpdate", "childVerify", "children"} {
		script.WriteString("  *" + name + "*)\n")
		if responses[name] == "FAIL" {
			script.WriteString("    echo 'boom' >&2; exit 1 ;;\n")
		} else {
			script.WriteString("    printf '%s' '" + responses[name] + "'; exit 0 ;;\n")
		}
	}
	script.WriteString("esac\necho \"unexpected glab call: $*\" >&2\nexit 2\n")
	return installGlab(t, script.String())
}

func TestGitLabCreateChildFailsBeforeCreatingWhenPrerequisitesAreMissing(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[string]string
		req       func(*port.IssueProviderCreateChildRequest)
		wantErr   string
	}{
		{"task type unavailable", map[string]string{"taskType": `{"data":{"namespace":{"workItemTypes":{"nodes":[]}}}}`}, nil, "Task work item type is not available"},
		{"task type lookup fails", map[string]string{"taskType": "FAIL"}, nil, "glab graphql failed"},
		{"label missing", map[string]string{"labelLookup": `{"data":{"project":{"labels":{"nodes":[]}}}}`}, nil, `label "bug" was not found`},
		{"label lookup fails", map[string]string{"labelLookup": "FAIL"}, nil, "glab graphql failed"},
		{"label title match is case-sensitive", map[string]string{"labelLookup": `{"data":{"project":{"labels":{"nodes":[{"id":"gid://x/1","title":"BUG"}]}}}}`}, nil, `label "bug" was not found`},
		{"user missing", map[string]string{"userLookup": `{"data":{"user":null}}`}, nil, `user "sample" was not found`},
		{"user lookup returns another account", map[string]string{"userLookup": `{"data":{"user":{"id":"gid://x/1","username":"other"}}}`}, nil, `user "sample" was not found`},
		{"user has no id", map[string]string{"userLookup": `{"data":{"user":{"id":"","username":"sample"}}}`}, nil, `user "sample" was not found`},
		{"user lookup fails", map[string]string{"userLookup": "FAIL"}, nil, "glab graphql failed"},
		{"at-sign assignee is looked up by bare username", map[string]string{"userLookup": `{"data":{"user":{"id":"gid://x/1","username":"@sample"}}}`}, func(r *port.IssueProviderCreateChildRequest) { r.Assignees = []string{"@sample"} }, `user "sample" was not found`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := installChildFlowGlab(t, tt.overrides)
			req := port.IssueProviderCreateChildRequest{Repo: repo, ParentIssueURL: "https://gitlab.com/acme/repo/-/issues/12", Title: "child", Labels: []string{"bug"}, Assignees: []string{"sample"}, Confirm: true}
			if tt.req != nil {
				tt.req(&req)
			}
			got, err := NewProvider().CreateChild(req)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
			if got.OK || got.ChildURL != "" {
				t.Fatalf("result = %+v", got)
			}
			if strings.Contains(gapCalls(t, repo), "workItemCreate") {
				t.Fatalf("no child may be created when a prerequisite fails:\n%s", gapCalls(t, repo))
			}
			if cerr, ok := errors.AsType[*port.IssueProviderCreateError](err); !ok || cerr.Invoked {
				t.Fatalf("a failed prerequisite must be a not-invoked create error, got %T %v", err, err)
			}
		})
	}
}

func TestGitLabCreateChildReportsCreateFailures(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[string]string
		wantErr   string
	}{
		{"workItemCreate returns errors", map[string]string{"workItemCreate": `{"data":{"workItemCreate":{"workItem":null,"errors":["title too long","bad type"]}}}`}, "workItemCreate failed: title too long, bad type"},
		{"workItemCreate returns no id", map[string]string{"workItemCreate": `{"data":{"workItemCreate":{"workItem":{"id":"  ","iid":"34"},"errors":[]}}}`}, "did not return child work item id"},
		{"workItemCreate transport failure", map[string]string{"workItemCreate": "FAIL"}, "glab graphql failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := installChildFlowGlab(t, tt.overrides)
			got, err := NewProvider().CreateChild(port.IssueProviderCreateChildRequest{Repo: repo, ParentIssueURL: "https://gitlab.com/acme/repo/-/issues/12", Title: "child", Confirm: true})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) || got.OK {
				t.Fatalf("got=%+v err=%v, want %q", got, err, tt.wantErr)
			}
			if strings.Contains(gapCalls(t, repo), "parentIid") {
				t.Fatal("a child that was not created must not be attached")
			}
		})
	}
}

func TestGitLabCreateChildReportsFollowUpFailuresWithTheCreatedChildURL(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[string]string
		wantErr   string
	}{
		{"parent lookup fails", map[string]string{"parentIid": "FAIL"}, "glab graphql failed"},
		{"parent id missing", map[string]string{"parentIid": `{"data":{"project":{"issue":{"id":""}}}}`}, "did not return work item id"},
		{"attach fails", map[string]string{"workItemHierarchyAddChildrenItems": "FAIL"}, "glab graphql failed"},
		{"attach returns errors", map[string]string{"workItemHierarchyAddChildrenItems": `{"data":{"workItemHierarchyAddChildrenItems":{"errors":["cycle","depth"]}}}`}, "hierarchy attach failed: cycle, depth"},
		{"children readback fails", map[string]string{"children": "FAIL"}, "glab graphql failed"},
		{"child absent from hierarchy", map[string]string{"children": `{"data":{"workItem":{"widgets":[{"type":"HIERARCHY","children":{"nodes":[{"id":"gid://gitlab/WorkItem/99","iid":"99"}]}}]}}}`}, "child hierarchy verification failed"},
		{"verification readback fails", map[string]string{"childVerify": "FAIL"}, "glab graphql failed"},
		{"label not applied", map[string]string{"childVerify": `{"data":{"workItem":{"iid":"34","webUrl":"` + gapChildWebURL + `","widgets":[{"type":"ASSIGNEES","assignees":{"nodes":[{"username":"sample"}]}},{"type":"LABELS","labels":{"nodes":[]}}]}}}`}, "missing labels: bug"},
		{"assignee not applied", map[string]string{"childVerify": `{"data":{"workItem":{"iid":"34","webUrl":"` + gapChildWebURL + `","widgets":[{"type":"LABELS","labels":{"nodes":[{"title":"bug"}]}},{"type":"ASSIGNEES","assignees":{"nodes":[{"username":"  "}]}}]}}}`}, "missing assignees: sample"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := installChildFlowGlab(t, tt.overrides)
			got, err := NewProvider().CreateChild(port.IssueProviderCreateChildRequest{
				Repo: repo, ParentIssueURL: "https://gitlab.com/acme/repo/-/issues/12", Title: "child",
				Labels: []string{"bug"}, Assignees: []string{"sample"}, Confirm: true,
			})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
			if !strings.Contains(err.Error(), "created child "+gapChildWebURL) {
				t.Fatalf("the created child's URL must be reported so it is not duplicated: %v", err)
			}
			if cerr, ok := errors.AsType[*port.IssueProviderCreateError](err); !ok || !cerr.Invoked {
				t.Fatalf("a follow-up failure after creation must be marked invoked: %v", err)
			}
			if got.OK || got.ChildURL != gapChildWebURL {
				t.Fatalf("result = %+v", got)
			}
		})
	}
}

func TestGitLabCreateChildRejectsBadRequestsBeforeAnyCall(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := NewProvider().CreateChild(port.IssueProviderCreateChildRequest{ParentIssueURL: gapParentURL, Title: "  ", Confirm: true}); err == nil || !strings.Contains(err.Error(), "child title is required") {
		t.Fatalf("title error = %v", err)
	}
	if _, err := NewProvider().CreateChild(port.IssueProviderCreateChildRequest{ParentIssueURL: "https://example.com/x", Title: "t", Confirm: true}); err == nil || !strings.Contains(err.Error(), "GitLab issue URL") {
		t.Fatalf("parent error = %v", err)
	}
}

func TestGitLabCreateChildBlankLabelsAndAssigneesAreSkipped(t *testing.T) {
	repo := installChildFlowGlab(t, nil)
	got, err := NewProvider().CreateChild(port.IssueProviderCreateChildRequest{
		Repo: repo, ParentIssueURL: "https://gitlab.com/acme/repo/-/issues/12", Title: "child",
		Labels: []string{" ", "bug"}, Assignees: []string{"  ", "sample"}, Confirm: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	calls := gapCalls(t, repo)
	if strings.Count(calls, "labelLookup") != 1 || strings.Count(calls, "userLookup") != 1 || !strings.Contains(calls, "username=sample") {
		t.Fatalf("blank entries must not trigger lookups:\n%s", calls)
	}
	if !got.OK || got.ChildNumber != "34" {
		t.Fatalf("result = %+v", got)
	}
}

func TestGitLabCloseChildClosesOpenChildAndPropagatesState(t *testing.T) {
	repo := installChildFlowGlab(t, nil)
	got, err := NewProvider().CloseChild(port.IssueProviderCloseChildRequest{
		Repo: repo, ParentIssueURL: "https://gitlab.com/acme/repo/-/issues/12", ChildURL: gapChildWebURL, Confirm: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.OK || !got.Closed || got.AlreadyClosed || got.State != "CLOSED" || got.ChildURL != gapChildWebURL {
		t.Fatalf("result = %+v", got)
	}
	if !strings.Contains(gapCalls(t, repo), "stateEvent: CLOSE") {
		t.Fatalf("an open child must be closed:\n%s", gapCalls(t, repo))
	}
}

func TestGitLabCloseChildRejectsMismatchedURLsBeforeAnyCall(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	tests := []struct{ name, parent, child, wantErr string }{
		{"bad parent", "https://example.com/x", gapChildURL, "GitLab issue URL"},
		{"child not a work item", gapParentURL, "https://gitlab.example.com/acme/repo/-/issues/34", "work item URL"},
		{"other project", gapParentURL, "https://gitlab.example.com/acme/other/-/work_items/34", "must match linked parent issue project"},
		{"other host", gapParentURL, "https://gitlab.other.com/acme/repo/-/work_items/34", "must match linked parent issue project"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewProvider().CloseChild(port.IssueProviderCloseChildRequest{ParentIssueURL: tt.parent, ChildURL: tt.child, Confirm: true})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) || got.OK {
				t.Fatalf("got=%+v err=%v, want %q", got, err, tt.wantErr)
			}
		})
	}
}

func TestGitLabCloseChildFailurePaths(t *testing.T) {
	tests := []struct {
		name      string
		overrides map[string]string
		wantErr   string
		mutated   bool
	}{
		{"parent lookup fails", map[string]string{"parentIid": "FAIL"}, "glab graphql failed", false},
		{"parent id missing", map[string]string{"parentIid": `{"data":{"project":{"issue":{"id":""}}}}`}, "did not return work item id", false},
		{"children readback fails", map[string]string{"children": "FAIL"}, "glab graphql failed", false},
		{"child not in hierarchy", map[string]string{"children": `{"data":{"workItem":{"widgets":[{"type":"HIERARCHY","children":{"nodes":[{"id":"gid://gitlab/WorkItem/99","iid":"99","state":"OPEN"}]}}]}}}`}, "child hierarchy verification failed", false},
		{"close transport failure", map[string]string{"workItemUpdate": "FAIL"}, "glab graphql failed", true},
		{"close returns errors", map[string]string{"workItemUpdate": `{"data":{"workItemUpdate":{"workItem":null,"errors":["not allowed","locked"]}}}`}, "workItemUpdate failed: not allowed, locked", true},
		{"close verification fails", map[string]string{"childCloseVerify": "FAIL"}, "glab graphql failed", true},
		{"child still open after close", map[string]string{"childCloseVerify": `{"data":{"workItem":{"id":"gid://gitlab/WorkItem/34","state":"OPEN"}}}`}, "close verification failed: state=OPEN", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := installChildFlowGlab(t, tt.overrides)
			got, err := NewProvider().CloseChild(port.IssueProviderCloseChildRequest{
				Repo: repo, ParentIssueURL: "https://gitlab.com/acme/repo/-/issues/12", ChildURL: gapChildWebURL, Confirm: true,
			})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) || got.OK || got.Closed {
				t.Fatalf("got=%+v err=%v, want %q", got, err, tt.wantErr)
			}
			if mutated := strings.Contains(gapCalls(t, repo), "stateEvent: CLOSE"); mutated != tt.mutated {
				t.Fatalf("close mutation attempted = %v, want %v", mutated, tt.mutated)
			}
		})
	}
}

func TestGitLabCloseChildPreviewStaysUnobservedWhenHierarchyCannotBeRead(t *testing.T) {
	for name, overrides := range map[string]map[string]string{
		"parent lookup fails": {"parentIid": "FAIL"},
		"parent id missing":   {"parentIid": `{"data":{"project":{"issue":{"id":""}}}}`},
		"children fail":       {"children": "FAIL"},
		"child not listed":    {"children": `{"data":{"workItem":{"widgets":[{"type":"HIERARCHY","children":{"nodes":[]}}]}}}`},
	} {
		t.Run(name, func(t *testing.T) {
			repo := installChildFlowGlab(t, overrides)
			got, err := NewProvider().CloseChild(port.IssueProviderCloseChildRequest{
				Repo: repo, ParentIssueURL: "https://gitlab.com/acme/repo/-/issues/12", ChildURL: gapChildWebURL,
			})
			if err != nil || !got.OK || got.Preview == "" {
				t.Fatalf("a preview must succeed even when the child is unobservable: %+v err=%v", got, err)
			}
			if got.HierarchyVerified || got.State != "" || got.AlreadyClosed || got.ChildURL != gapChildWebURL {
				t.Fatalf("an unobserved child must not claim hierarchy or state: %+v", got)
			}
		})
	}
}

func TestGitLabCreateChildAcceptsAtPrefixedAssigneeVerifiedByBareUsername(t *testing.T) {
	repo := installChildFlowGlab(t, nil)
	got, err := NewProvider().CreateChild(port.IssueProviderCreateChildRequest{
		Repo: repo, ParentIssueURL: "https://gitlab.com/acme/repo/-/issues/12", Title: "child",
		Assignees: []string{"@sample"}, Confirm: true,
	})
	if err != nil {
		t.Fatalf("an @-prefixed assignee is resolved by bare username and must verify against it: %v", err)
	}
	if !got.OK || got.ChildURL != gapChildWebURL || strings.Join(got.Assignees, ",") != "sample" {
		t.Fatalf("result = %+v", got)
	}
	if !strings.Contains(gapCalls(t, repo), "username=sample") {
		t.Fatalf("lookup must use the bare username:\n%s", gapCalls(t, repo))
	}
}
