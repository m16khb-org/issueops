package artifacttemplate

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// repoTemplates pairs every provider template in the repository with the
// contract it must follow.
var repoTemplates = []struct {
	path     string
	kind     IssueOpsArtifactKind
	template IssueOpsTemplateKind
}{
	{".github/ISSUE_TEMPLATE/implementation_task.yml", IssueOpsArtifactIssue, IssueOpsTemplateImplementationTask},
	{".github/ISSUE_TEMPLATE/feature_request.yml", IssueOpsArtifactIssue, IssueOpsTemplateFeature},
	{".github/ISSUE_TEMPLATE/bug_report.yml", IssueOpsArtifactIssue, IssueOpsTemplateBug},
	{".github/ISSUE_TEMPLATE/proposal.yml", IssueOpsArtifactIssue, IssueOpsTemplateProposal},
	{".github/ISSUE_TEMPLATE/child_task.yml", IssueOpsArtifactChild, IssueOpsTemplateChildTask},
	{".github/pull_request_template.md", IssueOpsArtifactPR, IssueOpsTemplatePullRequest},
	{".gitlab/issue_templates/implementation_task.md", IssueOpsArtifactIssue, IssueOpsTemplateImplementationTask},
	{".gitlab/issue_templates/feature_request.md", IssueOpsArtifactIssue, IssueOpsTemplateFeature},
	{".gitlab/issue_templates/bug_report.md", IssueOpsArtifactIssue, IssueOpsTemplateBug},
	{".gitlab/issue_templates/proposal.md", IssueOpsArtifactIssue, IssueOpsTemplateProposal},
	{".gitlab/issue_templates/child_task.md", IssueOpsArtifactChild, IssueOpsTemplateChildTask},
	{".gitlab/merge_request_templates/default.md", IssueOpsArtifactPR, IssueOpsTemplatePullRequest},
}

func TestRepositoryTemplatesMatchContract(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	for _, tc := range repoTemplates {
		content, err := os.ReadFile(filepath.Join(root, tc.path))
		if err != nil {
			t.Fatal(err)
		}
		if missing := missingTemplateTitles(string(content), tc.path, tc.kind, tc.template); len(missing) > 0 {
			t.Errorf("%s does not follow the %s contract; missing or out of order: %v", tc.path, tc.template, missing)
		}
	}

	// The comparison must notice a dropped required title.
	dropped := "## 요약\n\n## 변경 내용\n\n## 리뷰 포인트\n"
	if missing := missingTemplateTitles(dropped, "x.md", IssueOpsArtifactPR, IssueOpsTemplatePullRequest); !slices.Equal(missing, []string{"확인한 것"}) {
		t.Fatalf("missingTemplateTitles = %v, want [확인한 것]", missing)
	}
}

// missingTemplateTitles reports the contract's required titles that a
// template lacks, or that appear out of contract order. GitHub issue forms
// carry titles as textarea labels, Markdown templates as `## ` headings.
func missingTemplateTitles(content, path string, kind IssueOpsArtifactKind, template IssueOpsTemplateKind) []string {
	var titles []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(path, ".yml") {
			if title, ok := strings.CutPrefix(trimmed, "label: "); ok {
				titles = append(titles, strings.Trim(title, `"`))
			}
			continue
		}
		if title, ok := strings.CutPrefix(trimmed, "## "); ok {
			titles = append(titles, title)
		}
	}
	var missing []string
	next := 0
	for _, want := range RequiredSectionTitles(kind, template) {
		at := slices.Index(titles[next:], want)
		if at < 0 {
			missing = append(missing, want)
			continue
		}
		next += at + 1
	}
	return missing
}
