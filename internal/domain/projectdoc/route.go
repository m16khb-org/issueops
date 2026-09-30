package projectdoc

import (
	"path/filepath"
	"strings"
	"unicode"
)

type RouteDoc struct{ Rel, Reason string }

func NormalizeRouteTask(task string) string {
	task = strings.ToLower(strings.TrimSpace(task))
	if task == "" {
		return "general"
	}
	return task
}

// RouteDocsForTask expects the lowercased task selected by the caller.
func RouteDocsForTask(task string) []RouteDoc {
	base := []RouteDoc{{"AGENTS.md", "repo-level agent entrypoint and document router"}}
	p := func(name, reason string) RouteDoc {
		return RouteDoc{filepath.ToSlash(filepath.Join(ProjectDocsDir, name)), reason}
	}
	extra := []RouteDoc{}
	if strings.Contains(task, "gitlab") || strings.Contains(task, "github") ||
		strings.Contains(task, "glab") || strings.Contains(task, "gh issue") ||
		strings.Contains(task, "vcs") || strings.Contains(task, "merge request") ||
		strings.Contains(task, "pull request") || strings.Contains(task, "remote issue") {
		extra = append(extra, p("VCS.md", "verified VCS provider capabilities, exact request recipes, and CLI fallbacks"))
	}
	result := appendRouteDocsUnique(append([]RouteDoc(nil), base...), extra...)
	matched := false
	add := func(match bool, names ...RouteDoc) {
		if !match {
			return
		}
		matched = true
		result = appendRouteDocsUnique(result, names...)
	}
	add(hasTaskToken(task, "implement") || hasTaskToken(task, "implementation") || hasTaskToken(task, "edit") || strings.Contains(task, "구현"),
		p("CONSTITUTION.md", "source-of-truth and operating principles"),
		p("AGENT_WORKFLOW.md", "default start/work/verify/finish workflow"),
		p("CONVENTIONS.md", "general editing rules"),
		p("CAUTIONS.md", "known project risks"),
		p("TESTING.md", "default test design and verification guidance"))
	add(strings.Contains(task, "conflict") || strings.Contains(task, "constitution") || strings.Contains(task, "principle") || strings.Contains(task, "instruction") || strings.Contains(task, "session"),
		p("CONSTITUTION.md", "SessionStart baseline and source-of-truth priority"), p("CAUTIONS.md", "risks that may affect the decision"))
	add(strings.Contains(task, "caution") || strings.Contains(task, "risk") || strings.Contains(task, "false") || strings.Contains(task, "failure") || strings.Contains(task, "regression"),
		p("CAUTIONS.md", "known false cases, repeated failures, and risk notes"), p("TESTING.md", "test design rules and verification checks to prevent recurrence"), p("ADR.md", "decision context if the false case was caused by architecture"))
	add(strings.Contains(task, "adr") || strings.Contains(task, "decision") || strings.Contains(task, "alternative") || strings.Contains(task, "why"),
		p("ADR.md", "architecture decision rationale, rejected alternatives, and consequences"), p("ARCHITECTURE.md", "current structure affected by the decision"), p("CONSTITUTION.md", "principles that constrain decisions"))
	add(strings.Contains(task, "performance") || strings.Contains(task, "profiling") || strings.Contains(task, "성능") || strings.Contains(task, "프로파일링"),
		p("ARCHITECTURE.md", "performance-sensitive structure and boundaries"), p("TECH_STACK.md", "toolchain and profiling technology evidence"), p("TESTING.md", "performance verification and regression checks"))
	add(strings.Contains(task, "execution") || strings.Contains(task, "finish") || strings.Contains(task, "complete") || strings.Contains(task, "workflow"),
		p("AGENT_WORKFLOW.md", "agent start/work/verify/finish procedure"), p("TESTING.md", "verification evidence before completion"))
	add(strings.Contains(task, "commit") || hasTaskToken(task, "pr") || strings.Contains(task, "push"),
		p("COMMIT_POLICY.md", "commit message, staging, and verification policy"), p("TESTING.md", "checks to run before commit/PR"), p("CAUTIONS.md", "project-specific commit risks"))
	add(strings.Contains(task, "openapi") || strings.Contains(task, "swagger") || strings.Contains(task, "endpoint") || strings.Contains(task, "controller") || strings.Contains(task, "dto") || strings.Contains(task, "api doc") || strings.Contains(task, "api spec"),
		p("OPEN_API_SPEC.md", "project-specific OpenAPI/Swagger static and agent review prompt"), p("TESTING.md", "API documentation check commands and static-vs-agent boundary"), p("AGENT_WORKFLOW.md", "verification workflow"), p("CAUTIONS.md", "known API documentation risks"))
	strongTesting := strings.Contains(task, "test") || strings.Contains(task, "testing") || hasTaskToken(task, "ci")
	weakTesting := strings.Contains(task, "spec") || strings.Contains(task, "verify")
	add(strongTesting || (!matched && weakTesting),
		p("TESTING.md", "well/poorly structured test guidance plus test/build/lint command candidates"), p("TECH_STACK.md", "toolchain evidence"), p("AGENT_WORKFLOW.md", "verification workflow"), p("CAUTIONS.md", "known verification risks"))
	add(strings.Contains(task, "ux") || strings.Contains(task, "style") || strings.Contains(task, "styling") || strings.Contains(task, "css") || strings.Contains(task, "typography") || strings.Contains(task, "palette") || strings.Contains(task, "theme") || strings.Contains(task, "color") || strings.Contains(task, "accessibility") || strings.Contains(task, "a11y") || strings.Contains(task, "animation") || strings.Contains(task, "motion") || strings.Contains(task, "redesign"),
		p("DESIGN.md", "client design system: palette, typography, spacing, motion, accessibility, component states"), p("CONVENTIONS.md", "styling and component conventions"))
	add(strings.Contains(task, "architecture") || hasTaskToken(task, "design") || strings.Contains(task, "refactor"),
		p("ARCHITECTURE.md", "system structure and boundaries"), p("DESIGN.md", "client design system contract when present"), p("ADR.md", "past structure decisions and rejected alternatives"), p("CONSTITUTION.md", "decision priority and invariants"), p("CONVENTIONS.md", "editing and structure conventions"))
	add(strings.Contains(task, "dependency") || strings.Contains(task, "package") || strings.Contains(task, "upgrade") || strings.Contains(task, "stack"),
		p("TECH_STACK.md", "detected stack and package manager evidence"), p("CONVENTIONS.md", "dependency addition rules"), p("TESTING.md", "test design rules and checks after dependency changes"))
	strongOperations := strings.Contains(task, "deploy") || strings.Contains(task, "env") || strings.Contains(task, "operate")
	weakOperations := strings.Contains(task, "run") || strings.Contains(task, "local")
	add(strongOperations || (!matched && weakOperations),
		p("OPERATIONS.md", "local development, environment, and deployment guidance"), p("TECH_STACK.md", "toolchain evidence"), p("CAUTIONS.md", "operational risks"))
	if !matched {
		result = appendRouteDocsUnique(result,
			p("CONSTITUTION.md", "source-of-truth and operating principles"),
			p("AGENT_WORKFLOW.md", "default start/work/verify/finish workflow"),
			p("CONVENTIONS.md", "general editing rules"),
			p("CAUTIONS.md", "known project risks"),
			p("TESTING.md", "default test design and verification guidance"))
	}
	return result
}

func hasTaskToken(task, token string) bool {
	for _, field := range strings.FieldsFunc(task, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if field == token {
			return true
		}
	}
	return false
}

func appendRouteDocsUnique(dst []RouteDoc, docs ...RouteDoc) []RouteDoc {
	seen := make(map[string]bool, len(dst)+len(docs))
	for _, doc := range dst {
		seen[doc.Rel] = true
	}
	for _, doc := range docs {
		if !seen[doc.Rel] {
			dst = append(dst, doc)
			seen[doc.Rel] = true
		}
	}
	return dst
}
