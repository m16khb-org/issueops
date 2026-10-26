package remote

import (
	"strings"
)

// RepoPullRequestTemplatePaths lists, in lookup order, where a repository
// keeps the PR/MR description template the provider pre-fills. GitHub reads
// the root, docs/ and .github/; GitLab applies the Default template.
func RepoPullRequestTemplatePaths(provider string) []string {
	if strings.EqualFold(strings.TrimSpace(provider), "gitlab") {
		return []string{".gitlab/merge_request_templates/Default.md", ".gitlab/merge_request_templates/default.md"}
	}
	return []string{
		".github/pull_request_template.md", ".github/PULL_REQUEST_TEMPLATE.md",
		"pull_request_template.md", "PULL_REQUEST_TEMPLATE.md",
		"docs/pull_request_template.md", "docs/PULL_REQUEST_TEMPLATE.md",
	}
}

// TemplateSectionsMissing returns the Markdown headings of a repository
// template that the body does not carry, in template order. Headings match
// case-insensitively on their text.
func TemplateSectionsMissing(template, body string) []string {
	present := map[string]bool{}
	for _, heading := range markdownHeadings(body) {
		present[strings.ToLower(heading)] = true
	}
	var missing []string
	for _, heading := range markdownHeadings(template) {
		if !present[strings.ToLower(heading)] {
			missing = append(missing, heading)
		}
	}
	return missing
}

func markdownHeadings(text string) []string {
	var headings []string
	for line := range strings.SplitSeq(text, "\n") {
		trimmed := strings.TrimSpace(line)
		level := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
		if level == 0 || level > 6 || !strings.HasPrefix(trimmed[level:], " ") {
			continue
		}
		if heading := strings.TrimSpace(strings.TrimRight(trimmed[level:], "# ")); heading != "" {
			headings = append(headings, heading)
		}
	}
	return headings
}
