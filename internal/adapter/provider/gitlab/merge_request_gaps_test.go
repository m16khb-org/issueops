package gitlab

import (
	"errors"
	"strings"
	"testing"

	"issueops/internal/port"
)

const gapMRURL = "https://gitlab.com/acme/repo/-/merge_requests/16"

func gapMRView(overrides ...[2]string) string {
	fields := map[string]string{
		"web_url": `"` + gapMRURL + `"`, "title": `"MR"`, "description": `"body"`, "source_branch": `"feat/x"`,
		"target_branch": `"main"`, "draft": "true", "sha": `"` + strings.Repeat("a", 40) + `"`,
		"labels": `["bug"]`, "assignees": `[{"username":"sample"}]`, "source_project_id": "7", "target_project_id": "7",
	}
	for _, o := range overrides {
		fields[o[0]] = o[1]
	}
	var parts []string
	for k, v := range fields {
		parts = append(parts, `"`+k+`":`+v)
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func gapMRRequest(repo string) port.IssueProviderCreatePullRequestRequest {
	return port.IssueProviderCreatePullRequestRequest{
		Repo: repo, ProjectKey: "gitlab.com/acme/repo", Title: "MR", Body: "body", HeadBranch: "feat/x", BaseBranch: "main",
		Labels: []string{"bug"}, Assignees: []string{"sample"}, Draft: true, ExpectedHeadSHA: strings.Repeat("a", 40), Confirm: true,
	}
}

func TestGitLabCreatePullRequestRejectsInvalidRequestsBeforeAnyCall(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	base := gapMRRequest(t.TempDir())
	tests := []struct {
		name    string
		mutate  func(*port.IssueProviderCreatePullRequestRequest)
		wantErr string
	}{
		{"blank title", func(r *port.IssueProviderCreatePullRequestRequest) { r.Title = "  " }, "MR title is required"},
		{"malformed project authority", func(r *port.IssueProviderCreatePullRequestRequest) { r.ProjectKey = "gitlab.com/acme" }, "malformed"},
		{"uppercase project authority", func(r *port.IssueProviderCreatePullRequestRequest) { r.ProjectKey = "GitLab.com/acme/repo" }, "malformed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := base
			tt.mutate(&req)
			if _, err := NewProvider().CreatePullRequest(req); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestGitLabCreatePullRequestClassifiesGlabFailures(t *testing.T) {
	t.Run("missing glab is not invoked", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		_, err := NewProvider().CreatePullRequest(gapMRRequest(t.TempDir()))
		if cerr, ok := errors.AsType[*port.IssueProviderCreateError](err); !ok || cerr.Invoked {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("failure with a canonical URL keeps it", func(t *testing.T) {
		repo := installGlab(t, "printf '"+gapMRURL+"\\n'\necho timeout >&2\nexit 1\n")
		got, err := NewProvider().CreatePullRequest(gapMRRequest(repo))
		cerr, ok := errors.AsType[*port.IssueProviderCreateError](err)
		if !ok || !cerr.Invoked || got.URL != gapMRURL || !strings.Contains(err.Error(), "outcome unknown with a canonical URL returned separately; do not retry") {
			t.Fatalf("got=%+v err=%v", got, err)
		}
		if strings.Contains(gapCalls(t, repo), "mr view") {
			t.Fatal("a failed create must not be read back as if it succeeded")
		}
	})
	t.Run("failure with a non-canonical URL drops it", func(t *testing.T) {
		repo := installGlab(t, "printf 'https://gitlab.com/acme/repo/-/merge_requests/16?x=1\\n'\necho timeout >&2\nexit 1\n")
		got, err := NewProvider().CreatePullRequest(gapMRRequest(repo))
		cerr, ok := errors.AsType[*port.IssueProviderCreateError](err)
		if !ok || !cerr.Invoked || got.URL != "" || got.OK || !strings.Contains(err.Error(), "outcome unknown; do not retry") {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	})
	t.Run("success without a canonical URL needs reconciliation and no readback", func(t *testing.T) {
		repo := installGlab(t, "printf 'created\\n'\n")
		_, err := NewProvider().CreatePullRequest(gapMRRequest(repo))
		if err == nil || !strings.Contains(err.Error(), "created artifact URL unavailable; needs reconciliation; not retried") {
			t.Fatalf("error = %v", err)
		}
		if strings.Contains(gapCalls(t, repo), "mr view") {
			t.Fatal("no readback is possible without a canonical URL")
		}
	})
}

func TestGitLabCreatePullRequestReadbackFailuresNeedReconciliation(t *testing.T) {
	tests := []struct{ name, view, wantErr string }{
		{"view fails", "exit 1", "read back created merge request"},
		{"view malformed", "printf 'nope'; exit 0", "parse created merge request readback"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := installGlab(t, `case "$*" in
  "mr create"*) printf '`+gapMRURL+`\n'; exit 0 ;;
  "mr view"*) `+tt.view+` ;;
esac
exit 2
`)
			got, err := NewProvider().CreatePullRequest(gapMRRequest(repo))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) || !strings.Contains(err.Error(), "needs reconciliation") || got.OK || got.URL != gapMRURL {
				t.Fatalf("got=%+v err=%v", got, err)
			}
		})
	}
}

func TestGitLabCreatePullRequestRejectsEveryReadbackDrift(t *testing.T) {
	tests := []struct {
		name    string
		view    string
		mutate  func(*port.IssueProviderCreatePullRequestRequest)
		wantErr string
	}{
		{"web url", gapMRView([2]string{"web_url", `"https://gitlab.com/acme/repo/-/merge_requests/17"`}), nil, "identity or draft status"},
		{"source branch", gapMRView([2]string{"source_branch", `"other"`}), nil, "identity or draft status"},
		{"draft flag", gapMRView([2]string{"draft", "false"}), nil, "identity or draft status"},
		{"title", gapMRView([2]string{"title", `"changed"`}), nil, "title or rendered body"},
		{"body", gapMRView([2]string{"description", `"changed"`}), nil, "title or rendered body"},
		{"head sha", gapMRView([2]string{"sha", `"` + strings.Repeat("b", 40) + `"`}), nil, "head SHA"},
		{"labels", gapMRView([2]string{"labels", `["bug","extra"]`}), nil, "labels do not exactly match"},
		{"assignees", gapMRView([2]string{"assignees", `[]`}), nil, "assignees do not exactly match"},
		{"fork source project", gapMRView([2]string{"source_project_id", "9"}), nil, "source project"},
		{"missing project ids", gapMRView([2]string{"source_project_id", "0"}, [2]string{"target_project_id", "0"}), nil, "source project"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := installGlab(t, `case "$*" in
  "mr create"*) printf '`+gapMRURL+`\n'; exit 0 ;;
  "mr view"*) printf '%s' '`+tt.view+`'; exit 0 ;;
esac
exit 2
`)
			req := gapMRRequest(repo)
			if tt.mutate != nil {
				tt.mutate(&req)
			}
			_, err := NewProvider().CreatePullRequest(req)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) || !strings.Contains(err.Error(), "needs reconciliation") {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestGitLabCreatePullRequestSkipsOptionalChecksWhenNotRequested(t *testing.T) {
	// No project authority and no expected head: the readback is not compared on
	// either, and the project is derived from the created URL.
	repo := installGlab(t, `case "$*" in
  "mr create"*) printf '`+gapMRURL+`\n'; exit 0 ;;
  "mr view"*) printf '%s' '`+gapMRView([2]string{"sha", `"zzz"`}, [2]string{"source_project_id", "9"})+`'; exit 0 ;;
esac
exit 2
`)
	req := gapMRRequest(repo)
	req.ProjectKey, req.ExpectedHeadSHA = "", ""
	got, err := NewProvider().CreatePullRequest(req)
	if err != nil || !got.OK || got.URL != gapMRURL {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if !strings.Contains(gapCalls(t, repo), "mr view 16 --repo https://gitlab.com/acme/repo --output json") {
		t.Fatalf("readback must target the project derived from the created URL:\n%s", gapCalls(t, repo))
	}
}

func TestGitLabReconcilePullRequestRejectsUnusableInputsAndResponses(t *testing.T) {
	req := port.IssueProviderReconcilePullRequestRequest{ProjectKey: "gitlab.com/acme/repo", HeadBranch: "feat/x", BaseBranch: "main"}
	t.Run("no project authority", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		r := req
		r.ProjectKey = ""
		if _, err := NewProvider().ReconcilePullRequest(r); err == nil || !strings.Contains(err.Error(), "requires exact project authority") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("malformed project authority", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		r := req
		r.ProjectKey = "gitlab.com/acme"
		if _, err := NewProvider().ReconcilePullRequest(r); err == nil || !strings.Contains(err.Error(), "requires exact project authority") {
			t.Fatalf("error = %v", err)
		}
	})
	for name, tt := range map[string]struct{ body, wantErr string }{
		"glab failure":     {"exit 1", "list GitLab reconcile candidates"},
		"malformed":        {"printf 'nope'", "parse GitLab reconcile candidates"},
		"too many results": {"printf '[{},{},{}]'", "exceeded bound"},
	} {
		t.Run(name, func(t *testing.T) {
			r := req
			r.Repo = installGlab(t, tt.body+"\n")
			if _, err := NewProvider().ReconcilePullRequest(r); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestGitLabReconcilePullRequestReportsAuthoritativeZeroAndProjectsCandidates(t *testing.T) {
	r := port.IssueProviderReconcilePullRequestRequest{ProjectKey: "gitlab.com/acme/repo", HeadBranch: " feat/x "}
	t.Run("empty array is authoritative zero", func(t *testing.T) {
		r := r
		r.Repo = installGlab(t, "printf '[]'\n")
		got, err := NewProvider().ReconcilePullRequest(r)
		if err != nil || !got.AuthoritativeZero || len(got.Candidates) != 0 {
			t.Fatalf("got=%+v err=%v", got, err)
		}
		if !strings.Contains(gapCalls(t, r.Repo), "--source-branch feat/x ") {
			t.Fatalf("head branch must be trimmed in the search: %s", gapCalls(t, r.Repo))
		}
	})
	t.Run("candidate projection normalises project and flags forks", func(t *testing.T) {
		r := r
		r.Repo = installGlab(t, `printf '%s' '[{"web_url":"https://GitLab.com/Acme/Repo/-/merge_requests/3","title":" T ","description":"b","source_branch":"feat/x","target_branch":"main","state":" opened ","sha":"s","labels":["l"],"assignees":[{"username":"u"},{"username":" "}],"source_project_id":1,"target_project_id":2},{"web_url":"not a url %zz","source_project_id":5,"target_project_id":5}]'`+"\n")
		got, err := NewProvider().ReconcilePullRequest(r)
		if err != nil || got.AuthoritativeZero || len(got.Candidates) != 2 {
			t.Fatalf("got=%+v err=%v", got, err)
		}
		c := got.Candidates[0]
		if c.ProjectKey != "gitlab.com/acme/repo" || c.SourceProjectKey != "" || c.Title != "T" || c.State != "opened" ||
			c.BodySHA256 != gitLabBodySHA256("b") || strings.Join(c.Assignees, ",") != "u" || strings.Join(c.Labels, ",") != "l" {
			t.Fatalf("candidate = %+v", c)
		}
		if got.Candidates[1].ProjectKey != "" || got.Candidates[1].SourceProjectKey != "" {
			t.Fatalf("an unparsable URL must yield no project identity: %+v", got.Candidates[1])
		}
	})
}

func TestGitLabSourceProjectKeyRequiresMatchingPositiveIDs(t *testing.T) {
	for name, tt := range map[string]struct {
		source, target int64
		want           string
	}{
		"same project":   {7, 7, "k"},
		"fork":           {8, 7, ""},
		"unknown source": {0, 7, ""},
		"unknown target": {7, 0, ""},
		"negative ids":   {-1, -1, ""},
	} {
		t.Run(name, func(t *testing.T) {
			if got := gitLabSourceProjectKey("k", gitlabMergeRequestProjection{SourceProjectID: tt.source, TargetProjectID: tt.target}); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGlabCapabilityAtLeast182ParsesVersionBanners(t *testing.T) {
	for banner, want := range map[string]bool{
		"glab 1.82.0":                      true,
		"glab version 1.82.0 (2025-01-01)": true,
		"v2.0.0":                           true,
		"1.100.3":                          true,
		"glab 1.81.9":                      false,
		"glab 0.99.0":                      false,
		"glab dev":                         false,
		"":                                 false,
		"glab 1.82":                        false,
	} {
		if got := glabCapabilityAtLeast182(banner); got != want {
			t.Fatalf("%q = %v, want %v", banner, got, want)
		}
	}
}
