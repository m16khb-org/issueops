package projectdocs

import (
	projectdoc "issueops/internal/domain/projectdoc"

	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func RenderAgentsWithBlock(root, existing string) string {
	bullets := []string{
		"- Architecture or large design changes: " + projectdoc.ProjectDocsDir + "/ARCHITECTURE.md, " + projectdoc.ProjectDocsDir + "/CONSTITUTION.md",
		"- Testing or verification changes: " + projectdoc.ProjectDocsDir + "/TESTING.md",
		"- Endpoint/DTO/OpenAPI changes: " + projectdoc.ProjectDocsDir + "/OPEN_API_SPEC.md",
		"- Commit or PR work: " + projectdoc.ProjectDocsDir + "/COMMIT_POLICY.md",
		"- Code style or structure changes: " + projectdoc.ProjectDocsDir + "/CONVENTIONS.md",
		"- Dependency or tech-stack changes: " + projectdoc.ProjectDocsDir + "/TECH_STACK.md",
		"- Run, deploy, environment, or local development: " + projectdoc.ProjectDocsDir + "/OPERATIONS.md",
		"- Agent start, verification, and completion workflow: " + projectdoc.ProjectDocsDir + "/AGENT_WORKFLOW.md",
		"- Risky or recurring-failure work: " + projectdoc.ProjectDocsDir + "/CAUTIONS.md",
		"- Structural rationale, alternatives, and decisions: " + projectdoc.ProjectDocsDir + "/ADR.md",
		"- Session start, instruction conflicts, and principle decisions: " + projectdoc.ProjectDocsDir + "/CONSTITUTION.md",
	}
	if designDocExists(root) {
		bullets = append(bullets, "- UI, styling, or design-system changes: "+projectdoc.ProjectDocsDir+"/DESIGN.md (client repositories only)")
	}
	block := strings.TrimSpace(fmt.Sprintf(`%s
## issueops project docs

This repository uses issueops project docs. Read existing AGENTS.md rules first, then read only the additional documents relevant to the task.

%s
%s`, projectdoc.AgentsStartMarker, strings.Join(bullets, "\n"), projectdoc.AgentsEndMarker)) + "\n"
	path := filepath.Join(root, "AGENTS.md")
	b, err := os.ReadFile(path)
	if err != nil {
		return strings.TrimRight(projectdoc.BehavioralGuidelines, "\n") + "\n\n---\n\n" + block + "\n"
	}
	text := ensureBehavioralGuidelinesAtTop(string(b))
	start := strings.Index(text, projectdoc.AgentsStartMarker)
	end := strings.Index(text, projectdoc.AgentsEndMarker)
	if start >= 0 && end > start {
		end += len(projectdoc.AgentsEndMarker)
		return strings.TrimRight(text[:start], "\n") + "\n\n" + block + strings.TrimLeft(text[end:], "\n")
	}
	return strings.TrimRight(text, "\n") + "\n\n" + block
}

// designDocExists reports whether this repo carries a design-system doc:
// either the agent-facing .issueops/DESIGN.md or a curated root
// DESIGN.md that stays authoritative for the design system.
func designDocExists(root string) bool {
	for _, rel := range []string{
		filepath.Join(projectdoc.ProjectDocsDir, "DESIGN.md"),
		"DESIGN.md",
	} {
		if info, err := os.Stat(filepath.Join(root, rel)); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func ensureBehavioralGuidelinesAtTop(text string) string {
	trimmed := strings.TrimLeft(text, "\ufeff\n\r\t ")
	if strings.HasPrefix(trimmed, "# AGENTS.md\n\nBehavioral guidelines to reduce common LLM coding mistakes.") {
		return text
	}
	// issueops is a library applied to many repositories: when the
	// existing AGENTS.md already opens with its own heading and guidance,
	// that curated content stays authoritative. Do not stack the generic
	// behavioral template on top of repo-authored rules.
	if strings.HasPrefix(trimmed, "# ") {
		return text
	}
	return strings.TrimRight(projectdoc.BehavioralGuidelines, "\n") + "\n\n---\n\n" + strings.TrimLeft(text, "\n")
}
