package remote

import (
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
)

func TestPreparationProvider(t *testing.T) {
	for _, tc := range []struct {
		name, provider, url, want string
		wantErr                   bool
	}{
		{"explicit wins over url", "gitlab", "https://github.com/a/b/issues/1", "gitlab", false},
		{"inferred from github url", "", "https://github.com/a/b/issues/1", "github", false},
		{"inferred from gitlab url", "", "https://gitlab.com/a/b/-/issues/1", "gitlab", false},
		{"uninferable url rejected", "", "https://example.com/a", "", true},
		{"unknown explicit provider rejected", "bitbucket", "https://github.com/a/b/issues/1", "", true},
		{"provider is case sensitive", "GitHub", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := PreparationProvider(tc.provider, tc.url)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("got (%q,%v) want (%q,err=%v)", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestValidatePreparationIssueNumber(t *testing.T) {
	const issue = "https://github.com/a/b/issues/42"
	if err := ValidatePreparationIssueNumber(issue, "42-fix-login"); err != nil {
		t.Fatalf("matching prefix rejected: %v", err)
	}
	for _, branch := range []string{"fix-login", "421-fix", "42", "4-42-fix"} {
		err := ValidatePreparationIssueNumber(issue, branch)
		if err == nil || !strings.Contains(err.Error(), "42-fix-login") {
			t.Fatalf("branch %q err=%v, want prefix rejection with example", branch, err)
		}
	}
	if err := ValidatePreparationIssueNumber("https://example.com/none", "anything"); err != nil {
		t.Fatalf("issue without number must not constrain branch: %v", err)
	}
}

func TestPreparationCodeProject(t *testing.T) {
	const issue = "https://gitlab.example/planning/backlog/-/issues/9"
	const same = "gitlab.example/planning/backlog"
	const other = "gitlab.example/team/service"
	obsErr := errors.New("no origin")
	for _, tc := range []struct {
		name                string
		requested, observed string
		obsErr              error
		want                string
		wantErr             bool
	}{
		{"declared differing project is kept", " " + other + " ", "", nil, other, false},
		{"declared same as issue project collapses to empty", same, "", nil, "", false},
		{"declared non-canonical key rejected", "https://gitlab.example/team/service", "", nil, "", true},
		{"declared key wins over observed", other, "gitlab.example/x/y", nil, other, false},
		{"observed differing project adopted", "", " " + other + " ", nil, other, false},
		{"observed same as issue ignored", "", same, nil, "", false},
		{"observation error ignored", "", other, obsErr, "", false},
		{"empty observation ignored", "", "  ", nil, "", false},
		{"invalid observation ignored", "", "Not A Key", nil, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := PreparationCodeProject("gitlab", issue, tc.requested, tc.observed, tc.obsErr)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("got (%q,%v) want (%q,err=%v)", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestValidateChildLinkProvider(t *testing.T) {
	gh, gl := "https://github.com/a/b/issues/1", "https://gitlab.com/a/b/-/issues/1"
	if err := ValidateChildLinkProvider(gh, "https://github.com/a/b/issues/2"); err != nil {
		t.Fatalf("same provider rejected: %v", err)
	}
	if err := ValidateChildLinkProvider(gh, gl); err == nil {
		t.Fatal("cross-provider child accepted")
	}
	if err := ValidateChildLinkProvider("https://example.com/unknown", gl); err != nil {
		t.Fatalf("unknown parent provider must not constrain: %v", err)
	}
}

func TestCleanupBranchArtifactProjectKindAliases(t *testing.T) {
	for _, tc := range []struct{ url, provider, kind, want string }{
		{" https://github.com/Acme/Repo/pull/3 ", " GitHub ", " Pull_Request ", "github.com/acme/repo"},
		{"https://github.com/acme/repo/pull/3", "github", "pr", "github.com/acme/repo"},
		{"https://gitlab.example/g/s/p/-/merge_requests/3", "gitlab", "merge_request", "gitlab.example/g/s/p"},
	} {
		got, err := CleanupBranchArtifactProject(tc.url, tc.provider, tc.kind)
		if err != nil || got != tc.want {
			t.Fatalf("%+v got (%q,%v)", tc, got, err)
		}
	}
	for _, tc := range []struct{ url, provider, kind string }{
		{"https://github.com/acme/repo/pull/3", "github", "mr"},
		{"https://github.com/acme/repo/issues/3", "github", "pull_request"},
		{"not a url", "gitlab", "merge_request"},
	} {
		if got, err := CleanupBranchArtifactProject(tc.url, tc.provider, tc.kind); err == nil || got != "" {
			t.Fatalf("%+v got (%q,%v), want error", tc, got, err)
		}
	}
}

func TestValidateCleanupBranchOrigin(t *testing.T) {
	const key = "github.com/acme/repo"
	for _, origin := range []string{"git@github.com:Acme/Repo.git", " https://github.com/acme/repo.git ", "ssh://git@github.com/acme/repo"} {
		if err := ValidateCleanupBranchOrigin(key, origin, " GitHub "); err != nil {
			t.Fatalf("origin %q rejected: %v", origin, err)
		}
	}
	err := ValidateCleanupBranchOrigin(key, "git@github.com:acme/other.git", "github")
	if err == nil || !strings.Contains(err.Error(), "github.com/acme/other") {
		t.Fatalf("mismatch err=%v", err)
	}
	if err := ValidateCleanupBranchOrigin(key, "", "github"); err == nil {
		t.Fatal("empty origin accepted")
	}
	if err := ValidateCleanupBranchOrigin(key, "git@github.com:acme/repo.git", "bitbucket"); err == nil {
		t.Fatal("unsupported provider accepted")
	}
}

func TestValidateConfirmCreateMetadata(t *testing.T) {
	for _, tc := range []struct {
		name              string
		confirm           bool
		labels, assignees []string
		wantErr           string
	}{
		{"unconfirmed needs nothing", false, nil, nil, ""},
		{"confirmed complete", true, []string{"bug"}, []string{"me"}, ""},
		{"confirmed without labels", true, nil, []string{"me"}, "label"},
		{"blank labels do not count", true, []string{" ", "\x00"}, []string{"me"}, "label"},
		{"confirmed without assignees", true, []string{"bug"}, nil, "assignee"},
		{"blank assignees do not count", true, []string{"bug"}, []string{"  "}, "assignee"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateConfirmCreateMetadata(tc.confirm, tc.labels, tc.assignees)
			if tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("err=%v want %q", err, tc.wantErr)
			}
		})
	}
}

func TestValidProjectKey(t *testing.T) {
	for key, want := range map[string]bool{
		"github.com/acme/repo":         true,
		" gitlab.example/g/s/p ":       true,
		"host:8443/acme":               false, // colon is rejected
		"":                             false,
		"github.com":                   false,
		"GitHub.com/acme/repo":         false,
		"/github.com/acme":             false,
		"github.com/acme/":             false,
		"github.com//acme":             false,
		"github.com/acme/../repo":      false,
		"github.com/./repo":            false,
		"https://github.com/acme/repo": false,
		"github.com/acme/re po":        false,
		"github.com/acme?x":            false,
		"github.com/acme#x":            false,
		"github.com/acme\nrepo":        false,
	} {
		if got := ValidProjectKey(key); got != want {
			t.Errorf("ValidProjectKey(%q)=%v want %v", key, got, want)
		}
	}
}

func TestValidateChildMatchesParentRejectionBranches(t *testing.T) {
	const parent = "https://gitlab.example/g/p/-/issues/1"
	for _, tc := range []struct {
		name, parent, child, wantErr string
	}{
		{"same project", parent, "https://gitlab.example/g/p/-/issues/2", ""},
		{"work item accepted", parent, "https://gitlab.example/g/p/-/work_items/2", ""},
		{"unknown parent provider is unconstrained", "https://example.com/x", "whatever", ""},
		{"child without number", parent, "https://gitlab.example/g/p/-/issues/abc", "numeric gitlab"},
		{"child number but not canonical", "https://github.com/a/b/issues/1", "http://github.com/a/b/issues/2", "github issue URL"},
		{"child other project", parent, "https://gitlab.example/g/other/-/issues/2", "match linked parent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateChildMatchesParent(tc.parent, tc.child)
			if tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("err=%v want %q", err, tc.wantErr)
			}
		})
	}
}

func TestProjectKeyFromGitRemoteURL(t *testing.T) {
	for _, tc := range []struct {
		name, raw, provider, want string
		wantErr                   bool
	}{
		{"scp github", "git@GitHub.com:Acme/Repo.git", "github", "github.com/acme/repo", false},
		{"https non-default port kept", "https://gitlab.example:8443/g/s/p.git", "gitlab", "gitlab.example:8443/g/s/p", false},
		{"https default port dropped", "https://gitlab.example:443/g/p", "gitlab", "gitlab.example/g/p", false},
		{"ssh with port drops port", "ssh://git@gitlab.example:2222/g/p.git", "gitlab", "gitlab.example/g/p", false},
		{"ssh without userinfo", "ssh://github.com/a/b", "github", "github.com/a/b", false},
		{"github needs exactly two segments", "https://github.com/a/b/c", "github", "", true},
		{"gitlab on github host rejected", "https://github.com/a/b", "gitlab", "", true},
		{"gitlab needs two segments", "https://gitlab.example/solo", "gitlab", "", true},
		{"unsupported provider", "https://github.com/a/b", "gitea", "", true},
		{"empty", "  ", "github", "", true},
		{"embedded whitespace", "https://github.com/a /b", "github", "", true},
		{"http scheme", "http://github.com/a/b", "github", "", true},
		{"git scheme", "git://github.com/a/b", "github", "", true},
		{"https userinfo", "https://tok@github.com/a/b", "github", "", true},
		{"https query", "https://github.com/a/b?x=1", "github", "", true},
		{"https fragment", "https://github.com/a/b#x", "github", "", true},
		{"ssh password", "ssh://git:pw@github.com/a/b", "github", "", true},
		{"https without host", "https:///a/b", "github", "", true},
		{"garbage", "just-a-path", "github", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ProjectKeyFromGitRemoteURL(tc.raw, tc.provider)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("got (%q,%v) want (%q,err=%v)", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestProjectKeyRejectsNonCanonicalOrWrongKind(t *testing.T) {
	for _, tc := range []struct{ url, provider, kind, want string }{
		{"https://github.com/Acme/Repo/issues/1", "github", "issue", "github.com/acme/repo"},
		{"https://github.com/acme/repo/pull/1", "github", "pr", "github.com/acme/repo"},
		{"https://github.com/acme/repo/pull/1", "github", "issue", ""},
		{"https://github.com/acme/repo/issues/x", "github", "issue", ""},
		{"http://github.com/acme/repo/issues/1", "github", "issue", ""},
		{"https://u@github.com/acme/repo/issues/1", "github", "issue", ""},
		{"https://github.com/acme/repo/issues/1?x=1", "github", "issue", ""},
		{"https://github.com/acme/repo/issues/1#c", "github", "issue", ""},
		{"https://github.com/acme/repo/-/issues/1", "gitlab", "issue", ""},
		{"https://gitlab.example/g/p/-/issues/1", "gitlab", "pr", ""},
		{"https://gitlab.example/g/p/-/merge_requests/1", "gitlab", "issue", ""},
		{"https://gitlab.example/g/p/-/merge_requests/1", "gitlab", "mr", "gitlab.example/g/p"},
		{"https://gitlab.example:8443/g/p/-/issues/1", "gitlab", "issue", "gitlab.example:8443/g/p"},
		{"https://[::1]:8443/g/p/-/issues/1", "gitlab", "issue", "[::1]:8443/g/p"},
		{"https://[::1]/g/p/-/issues/1", "gitlab", "issue", "[::1]/g/p"},
		{"https://gitlab.example/g/p/-/issues/1", "bitbucket", "issue", ""},
	} {
		if got := ProjectKey(tc.url, tc.provider, tc.kind); got != tc.want {
			t.Errorf("ProjectKey(%q,%s,%s)=%q want %q", tc.url, tc.provider, tc.kind, got, tc.want)
		}
	}
}

func TestIssueNumberAndArtifactPaths(t *testing.T) {
	for url, want := range map[string]string{
		"https://github.com/a/b/issues/42":          "42",
		"https://gitlab.example/g/p/-/work_items/7": "7",
		"https://gitlab.example/g/p/-/issues/7a":    "",
		"https://github.com/a/b/issues/":            "",
		"https://github.com/a/b/pull/3":             "",
		"://bad":                                    "",
	} {
		if got := IssueNumber(url); got != want {
			t.Errorf("IssueNumber(%q)=%q want %q", url, got, want)
		}
	}
	if got := IssueArtifactDir("https://github.com/a/b/issues/42"); got != ".issueops/issues/42/artifact" {
		t.Fatalf("dir=%q", got)
	}
	if got := IssueArtifactDir("https://github.com/a/b/pull/42"); got != "" {
		t.Fatalf("numberless issue must have no artifact dir, got %q", got)
	}
	const issue = "https://github.com/a/b/issues/42"
	for _, name := range TrackedMaterialNames {
		for _, p := range []string{".issueops/issues/42/" + name, `.issueops\issues\42\` + name} {
			if !IsTrackedMaterialPath(issue, p) {
				t.Errorf("%q should be tracked", p)
			}
		}
	}
	for _, p := range []string{".issueops/issues/42/gates.md", ".issueops/issues/43/plan.md", "x/.issueops/issues/42/plan.md", ".issueops/issues/42/plan.md/extra"} {
		if IsTrackedMaterialPath(issue, p) {
			t.Errorf("%q should not be tracked", p)
		}
	}
	if IsTrackedMaterialPath("https://example.com/none", ".issueops/issues//plan.md") {
		t.Fatal("numberless issue must track nothing")
	}
}

func TestRenderIssueOpsRemoteJudgePrompt(t *testing.T) {
	req := IssueOpsRemoteLLMJudgeRequest{Request: IssueOpsRemoteScoringRequest{
		Provider: "gitlab", Issue: IssueOpsRemoteArtifact{Title: "UNIQUE-TITLE-XYZ", Body: "body"},
		IssueCandidates: []IssueOpsRemoteIssueCandidate{{ID: "#9", Title: "cand"}},
	}}
	res, err := RenderIssueOpsRemoteJudgePrompt(req)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || !res.ReadOnly || res.ExecutionClass != "background_join" || res.JoinBefore != "remote_artifact_write" {
		t.Fatalf("result=%+v", res)
	}
	for _, want := range []string{"UNIQUE-TITLE-XYZ", `"provider":"gitlab"`, "remote_artifact_write", "selected_related_issues"} {
		if !strings.Contains(res.Prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	plain, err := RenderIssueOpsRemoteLLMJudgePrompt(req)
	if err != nil || plain != res.Prompt {
		t.Fatalf("plain prompt differs from wrapped prompt (err=%v)", err)
	}
}

func TestRenderIssueOpsRemoteJudgePromptRejectsUnmarshalableScore(t *testing.T) {
	// NaN cannot be encoded as JSON, so building the request payload must fail
	// and neither renderer may return a half-built prompt.
	nan := zeroOverZero()
	req := IssueOpsRemoteLLMJudgeRequest{Request: IssueOpsRemoteScoringRequest{
		Issue:           IssueOpsRemoteArtifact{Title: "t"},
		IssueCandidates: []IssueOpsRemoteIssueCandidate{{ID: "1", Score: &nan}},
	}}
	if p, err := RenderIssueOpsRemoteLLMJudgePrompt(req); err == nil || p != "" {
		t.Fatalf("llm prompt=(%q,%v)", p, err)
	}
	if res, err := RenderIssueOpsRemoteJudgePrompt(req); err == nil || res.OK || res.Prompt != "" {
		t.Fatalf("judge prompt=(%+v,%v)", res, err)
	}
}

func zeroOverZero() float64 {
	zero := 0.0
	return zero / zero
}

func TestDecodeIssueOpsRemoteJudgeJSONRejectsContractViolations(t *testing.T) {
	valid := map[string]any{
		"ok": true, "provider": "github", "threshold": 0.7,
		"execution_class": "background_join", "read_only": true, "join_before": "remote_artifact_write",
		"selected_related_issues": []any{}, "rejected_related_issues": []any{},
		"selected_labels": []any{}, "rejected_labels": []any{},
		"apply_instructions": []any{}, "warnings": []any{},
	}
	encode := func(key string, value any) []byte {
		m := map[string]any{}
		maps.Copy(m, valid)
		m[key] = value
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	if _, err := DecodeIssueOpsRemoteJudgeJSON(encode("provider", "github")); err != nil {
		t.Fatalf("baseline rejected: %v", err)
	}
	for _, tc := range []struct {
		key     string
		value   any
		wantMsg string
	}{
		{"ok", false, "not ok"},
		{"execution_class", "foreground", "execution_class"},
		{"execution_class", "", "execution_class"},
		{"read_only", false, "read_only"},
		{"join_before", "never", "join_before"},
		{"join_before", "", "join_before"},
	} {
		_, err := DecodeIssueOpsRemoteJudgeJSON(encode(tc.key, tc.value))
		if err == nil || !strings.Contains(err.Error(), tc.wantMsg) {
			t.Errorf("%s=%v err=%v want %q", tc.key, tc.value, err, tc.wantMsg)
		}
	}
	if _, err := DecodeIssueOpsRemoteJudgeJSON([]byte("  ")); err == nil {
		t.Fatal("empty output accepted")
	}
}

func TestScoreIssueOpsRemoteCandidatesExplicitScoresAndOrdering(t *testing.T) {
	score := func(v float64) *float64 { return &v }
	res, err := ScoreIssueOpsRemoteCandidates(IssueOpsRemoteScoringRequest{
		Provider: "gitlab", Threshold: 0.5,
		Issue: IssueOpsRemoteArtifact{Title: "login crash"},
		IssueCandidates: []IssueOpsRemoteIssueCandidate{
			{ID: "b", Score: score(0.8), URL: " https://gitlab.example/g/p/-/issues/2 "},
			{ID: "a", Score: score(0.8)},
			{ID: "hi", Score: score(7)},
			{ID: "lo", Score: score(-3)},
			{Title: "title-only-ref", Score: score(0.9)},
		},
		LabelCandidates: []IssueOpsRemoteLabelCandidate{
			{Name: "zeta", Score: score(0.6)},
			{Name: "alpha", Score: score(0.6)},
			{Name: "weak", Score: score(0.1)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, it := range res.SelectedRelatedIssues {
		ids = append(ids, it.ID+"|"+it.Title)
		if it.Score < 0 || it.Score > 1 {
			t.Errorf("score %v not clamped", it.Score)
		}
	}
	// score desc; ties broken by id ascending; 7 clamps to 1.0 and sorts first.
	if want := []string{"hi|", "|title-only-ref", "a|", "b|"}; !slices.Equal(ids, want) {
		t.Fatalf("selected order=%v want %v", ids, want)
	}
	if len(res.RejectedRelatedIssues) != 1 || res.RejectedRelatedIssues[0].ID != "lo" || res.RejectedRelatedIssues[0].Score != 0 ||
		res.RejectedRelatedIssues[0].RejectReason == "" {
		t.Fatalf("rejected=%+v", res.RejectedRelatedIssues)
	}
	byID := map[string]IssueOpsRemoteScoredItem{}
	for _, it := range res.SelectedRelatedIssues {
		byID[it.ID+it.Title] = it
	}
	if got := byID["b"].ApplyHint; got != "attach as GitLab linked item via issue links API: https://gitlab.example/g/p/-/issues/2" {
		t.Errorf("url hint=%q", got)
	}
	if got := byID["a"].ApplyHint; got != "attach as GitLab linked item via issue links API: a" {
		t.Errorf("id hint=%q", got)
	}
	if got := byID["title-only-ref"].ApplyHint; got != "attach selected related issue as a GitLab linked item via the issue links API" {
		t.Errorf("bare hint=%q", got)
	}
	var names []string
	for _, it := range res.SelectedLabels {
		names = append(names, it.Name)
	}
	if !slices.Equal(names, []string{"alpha", "zeta"}) {
		t.Fatalf("label order=%v", names)
	}
	if res.SelectedLabels[0].ApplyHint != "apply GitLab label: alpha" {
		t.Fatalf("label hint=%q", res.SelectedLabels[0].ApplyHint)
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("unexpected warnings %v", res.Warnings)
	}
}

func TestScoreIssueOpsRemoteCandidatesGitHubHints(t *testing.T) {
	one := 1.0
	res, err := ScoreIssueOpsRemoteCandidates(IssueOpsRemoteScoringRequest{
		Issue: IssueOpsRemoteArtifact{Body: "x"},
		IssueCandidates: []IssueOpsRemoteIssueCandidate{
			{ID: "#1", Score: &one, URL: "https://github.com/a/b/issues/1"},
			{ID: "#2", Score: &one}, {Title: "no ref", Score: &one},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	hints := map[string]string{}
	for _, it := range res.SelectedRelatedIssues {
		hints[it.ID+it.Title] = it.ApplyHint
	}
	if hints["#1"] != "link in issue body: https://github.com/a/b/issues/1" || hints["#2"] != "link in issue body: #2" ||
		hints["no ref"] != "link selected related issue in issue body" {
		t.Fatalf("hints=%v", hints)
	}
}

func TestScoreIssueOpsRemoteCandidatesRejectsInvalidRequests(t *testing.T) {
	for _, tc := range []struct {
		name     string
		req      IssueOpsRemoteScoringRequest
		wantMsg  string
		provider string
	}{
		{"unsupported provider", IssueOpsRemoteScoringRequest{Provider: " Bitbucket ", Issue: IssueOpsRemoteArtifact{Title: "t"}}, "unsupported", "bitbucket"},
		{"provider falls back to issue provider", IssueOpsRemoteScoringRequest{Issue: IssueOpsRemoteArtifact{Provider: "svn", Title: "t"}}, "unsupported", "svn"},
		{"threshold above one", IssueOpsRemoteScoringRequest{Threshold: 1.01, Issue: IssueOpsRemoteArtifact{Title: "t"}}, "threshold", "github"},
		{"blank issue", IssueOpsRemoteScoringRequest{Issue: IssueOpsRemoteArtifact{Title: " ", Body: "\n"}}, "issue.title or issue.body", "github"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := ScoreIssueOpsRemoteCandidates(tc.req)
			if err == nil || !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("err=%v want %q", err, tc.wantMsg)
			}
			if res.OK || res.Provider != tc.provider {
				t.Fatalf("result=%+v", res)
			}
		})
	}
}

func TestScoreIssueOpsRemoteCandidatesHeuristicScoringWithDefaultThreshold(t *testing.T) {
	res, err := ScoreIssueOpsRemoteCandidates(IssueOpsRemoteScoringRequest{
		Issue: IssueOpsRemoteArtifact{Title: "login bug failure", Body: "login page shows bug failure after sign in"},
		IssueCandidates: []IssueOpsRemoteIssueCandidate{
			{ID: "#1", Title: "login bug failure", Body: "login page shows bug failure after sign in", Labels: []string{"bug"}},
			{ID: "#2", Title: "unrelated billing export", Body: "csv invoices"},
		},
		LabelCandidates: []IssueOpsRemoteLabelCandidate{
			{Name: "bug", Description: "bug failure defect"},
			{Name: "documentation", Description: "docs"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Threshold != 0.70 {
		t.Fatalf("default threshold=%v", res.Threshold)
	}
	if len(res.SelectedRelatedIssues) != 1 || res.SelectedRelatedIssues[0].ID != "#1" {
		t.Fatalf("selected issues=%+v", res.SelectedRelatedIssues)
	}
	if len(res.RejectedRelatedIssues) != 1 || res.RejectedRelatedIssues[0].ID != "#2" || res.RejectedRelatedIssues[0].ApplyHint != "" {
		t.Fatalf("rejected issues=%+v", res.RejectedRelatedIssues)
	}
	if len(res.SelectedLabels) != 1 || res.SelectedLabels[0].Name != "bug" {
		t.Fatalf("selected labels=%+v", res.SelectedLabels)
	}
	if len(res.RejectedLabels) != 1 || res.RejectedLabels[0].Name != "documentation" {
		t.Fatalf("rejected labels=%+v", res.RejectedLabels)
	}
	if len(res.SelectedRelatedIssues[0].Evidence) != 3 || len(res.SelectedLabels[0].Evidence) != 2 {
		t.Fatalf("heuristic evidence missing: %+v %+v", res.SelectedRelatedIssues[0].Evidence, res.SelectedLabels[0].Evidence)
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("warnings=%v", res.Warnings)
	}
}
