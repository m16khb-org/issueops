package issueopsremote

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/domain/artifacttemplate"
)

func TestTemplateScoreSummaryReadsJSONOrAuthoredText(t *testing.T) {
	resolver := NewTemplateBodyResolver(os.ReadFile)
	for _, tc := range []struct{ name, body, want string }{
		{"scored JSON", `{"threshold":0.7,"selected_labels":[{"name":" bug ","score":0.95}]}`, "threshold 0.70\n선택 라벨: bug(0.95)"},
		{"authored summary", "  human summary\n", "human summary"},
		{"empty JSON", `{}`, "threshold 0.00"},
		{"null JSON", `null`, "threshold 0.00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "scores")
			if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := resolver.ScoreSummary(" " + path + " ")
			if err != nil || got != tc.want {
				t.Fatalf("summary = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	if got, err := resolver.ScoreSummary("  "); err != nil || got != "" {
		t.Fatalf("no file = %q, %v", got, err)
	}
	if _, err := resolver.ScoreSummary(filepath.Join(t.TempDir(), "missing")); !os.IsNotExist(err) {
		t.Fatalf("missing file = %v", err)
	}
}

func TestTemplateResolveRefusesBadInputsBeforeRendering(t *testing.T) {
	resolver := NewTemplateBodyResolver(os.ReadFile)
	for _, tc := range []struct {
		name string
		req  TemplateBodyRequest
		want string
	}{
		{"body and file", TemplateBodyRequest{Kind: artifacttemplate.IssueOpsArtifactIssue, Body: "a", BodyFile: "b"}, "mutually exclusive"},
		{"confirm without issue template", TemplateBodyRequest{Kind: artifacttemplate.IssueOpsArtifactIssue, Confirm: true}, "--template is required"},
		{"malformed field", TemplateBodyRequest{Kind: artifacttemplate.IssueOpsArtifactIssue, Fields: []string{"no-equals"}}, "key=value"},
		{"missing score file", TemplateBodyRequest{Kind: artifacttemplate.IssueOpsArtifactIssue, ScoreFile: "/definitely/missing/score.json"}, "no such file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := resolver.Resolve(tc.req); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestTemplateResolveEmptyIssuePreviewReportsFindingsButConfirmRefuses(t *testing.T) {
	resolver := NewTemplateBodyResolver(os.ReadFile)
	req := TemplateBodyRequest{Kind: artifacttemplate.IssueOpsArtifactIssue, Template: "implementation_task", Title: "제목"}
	preview, err := resolver.Resolve(req)
	if err != nil || preview.Body != "" || preview.Readability.OK {
		t.Fatalf("an empty authored body is judged as empty, not replaced by the skeleton: %+v err=%v", preview, err)
	}
	req.Confirm = true
	refused, err := resolver.Resolve(req)
	if err == nil || !strings.Contains(err.Error(), "readability check failed") || refused.Readability.OK {
		t.Fatalf("confirm must refuse an unreadable body: %+v err=%v", refused, err)
	}
}

// An agent's untemplated preview of a body missing 배경/범위 came back clean,
// then --confirm refused for the template and its required sections.
func TestTemplateResolveUntemplatedIssuePreviewIsNotAPass(t *testing.T) {
	resolver := NewTemplateBodyResolver(os.ReadFile)
	preview, err := resolver.Resolve(TemplateBodyRequest{Kind: artifacttemplate.IssueOpsArtifactIssue, Title: "제목", Body: "## 요약\n\n짧은 요약입니다.\n"})
	if err != nil || preview.Readability.OK || len(preview.Readability.Critical) == 0 || preview.Readability.Critical[len(preview.Readability.Critical)-1].Code != "template_missing" {
		t.Fatalf("preview = %+v err=%v", preview.Readability, err)
	}
	child, err := resolver.Resolve(TemplateBodyRequest{Kind: artifacttemplate.IssueOpsArtifactChild, Title: "제목", Body: "## 요약\n\n짧은 요약입니다.\n"})
	for _, finding := range child.Readability.Critical {
		if err != nil || finding.Code == "template_missing" {
			t.Fatalf("child kinds have a single template: %+v err=%v", child.Readability, err)
		}
	}
}
