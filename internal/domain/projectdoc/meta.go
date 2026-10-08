package projectdoc

import "strings"

// docMetaDescriptions is the canonical, name-keyed metadata for standard project
// docs: one short sentence naming WHAT CATEGORY of information each doc holds
// (not a summary of its current content) and WHEN an agent should read it, as
// "<what>; read <when>.". Same doc name => same description in every repo, and
// it stays fixed across bootstrap and bootstrap --sync. It is rendered as
// SKILL.md-style YAML frontmatter at the top of each doc so both humans and the
// project-doc catalog read the same source.
var docMetaDescriptions = map[string]string{
	"ARCHITECTURE.md":   "System structure and component boundaries; read before adding a component or moving a responsibility.",
	"ADR.md":            "Accepted structural decisions and their rationale; read before reversing or extending a design choice.",
	"CONSTITUTION.md":   "Instruction priority, safety, and accuracy principles; read when rules conflict or an action is risky.",
	"CONVENTIONS.md":    "Coding conventions and layer boundaries; read before writing or restructuring code.",
	"TECH_STACK.md":     "Chosen languages, runtimes, and tools; read before adding a dependency or tool.",
	"TESTING.md":        "Verification standards and required checks; read before writing tests or claiming work is verified.",
	"COMMIT_POLICY.md":  "Commit message format and scope rules; read before committing.",
	"CAUTIONS.md":       "Recurring mistakes and operational pitfalls; read before a risky change or when a failure repeats.",
	"OPERATIONS.md":     "Install, runtime, and operating procedures; read before running, deploying, or troubleshooting.",
	"OPEN_API_SPEC.md":  "Endpoint, DTO, and OpenAPI documentation gates; read before changing an API contract.",
	"AGENT_WORKFLOW.md": "Agent start, execution, verification, and completion flow; read when starting or handing off a task.",
	"VCS.md":            "Verified VCS provider capabilities and CLI recipes; read before issue, PR, MR, or branch operations.",
	"DESIGN.md":         "Client design system tokens and component states; read before changing UI.",
}

// DocMetaDescription returns the canonical metadata description for a standard
// project doc filename, and whether one exists.
func DocMetaDescription(name string) (string, bool) {
	desc, ok := docMetaDescriptions[name]
	return desc, ok
}

// parseDocFrontmatter extracts a leading SKILL.md-style frontmatter block
// (--- ... ---) from the very top of content. It returns the name and
// description fields, the body after the block, and whether a block was present.
func ParseFrontmatter(content string) (name, description, body string, ok bool) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", content, false
	}
	closeIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			closeIdx = i
			break
		}
	}
	if closeIdx < 0 {
		return "", "", content, false
	}
	for _, line := range lines[1:closeIdx] {
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		switch strings.TrimSpace(key) {
		case "name":
			name = strings.TrimSpace(value)
		case "description":
			description = strings.TrimSpace(value)
		}
	}
	body = strings.TrimLeft(strings.Join(lines[closeIdx+1:], "\n"), "\n")
	return name, description, body, true
}

// renderDocMetaFrontmatter renders the canonical frontmatter block for a doc.
func renderDocMetaFrontmatter(name, description string) string {
	return "---\nname: " + name + "\ndescription: " + description + "\n---\n"
}

// ensureDocMetaFrontmatter guarantees content begins with the canonical meta
// frontmatter for the given doc name while preserving the existing body. An
// existing frontmatter block is replaced; otherwise one is prepended. Content is
// returned unchanged when the doc has no canonical metadata. The operation is
// idempotent: applying it twice yields identical output.
func EnsureMetaFrontmatter(name, content string) string {
	desc, ok := DocMetaDescription(name)
	if !ok {
		return content
	}
	_, _, body, hadFrontmatter := ParseFrontmatter(content)
	if !hadFrontmatter {
		body = content
	}
	block := renderDocMetaFrontmatter(name, desc)
	if strings.TrimSpace(body) == "" {
		return block
	}
	return block + "\n" + strings.TrimLeft(body, "\n")
}
