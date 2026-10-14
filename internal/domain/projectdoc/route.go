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

type routeDocSpec struct{ name, reason string }

// routeRule selects docs when any substring of the task or any whole token
// matches. weakSubs only match while no earlier rule has matched.
type routeRule struct {
	subs     []string
	tokens   []string
	weakSubs []string
	docs     []routeDocSpec
}

var defaultRouteDocs = []routeDocSpec{
	{"CONSTITUTION.md", "source-of-truth and operating principles"},
	{"AGENT_WORKFLOW.md", "default start/work/verify/finish workflow"},
	{"CONVENTIONS.md", "general editing rules"},
	{"CAUTIONS.md", "known project risks"},
	{"TESTING.md", "default test design and verification guidance"},
}

var vcsRouteSubs = []string{"gitlab", "github", "glab", "gh issue", "vcs", "merge request", "pull request", "remote issue", "branch", "rebase", "브랜치", "머지", "병합", "리베이스"}

var routeRules = []routeRule{
	{tokens: []string{"implement", "implementation", "edit"}, subs: []string{"구현"}, docs: defaultRouteDocs},
	{subs: []string{"conflict", "constitution", "principle", "instruction", "session"}, docs: []routeDocSpec{
		{"CONSTITUTION.md", "SessionStart baseline and source-of-truth priority"},
		{"CAUTIONS.md", "risks that may affect the decision"}}},
	{subs: []string{"caution", "risk", "false", "failure", "regression", "실패", "회귀", "주의"}, docs: []routeDocSpec{
		{"CAUTIONS.md", "known false cases, repeated failures, and risk notes"},
		{"TESTING.md", "test design rules and verification checks to prevent recurrence"},
		{"ADR.md", "decision context if the false case was caused by architecture"}}},
	{subs: []string{"브랜치", "머지", "병합", "리베이스", "워크트리"}, tokens: []string{"branch", "branches", "rebase", "squash", "worktree", "worktrees", "cleanup"}, docs: []routeDocSpec{
		{"CAUTIONS.md", "recorded branch, merge, and worktree cleanup pitfalls"}}},
	{subs: []string{"adr", "decision", "alternative", "why", "결정", "대안"}, docs: []routeDocSpec{
		{"ADR.md", "architecture decision rationale, rejected alternatives, and consequences"},
		{"ARCHITECTURE.md", "current structure affected by the decision"},
		{"CONSTITUTION.md", "principles that constrain decisions"}}},
	{subs: []string{"performance", "profiling", "성능", "프로파일링"}, docs: []routeDocSpec{
		{"ARCHITECTURE.md", "performance-sensitive structure and boundaries"},
		{"TECH_STACK.md", "toolchain and profiling technology evidence"},
		{"TESTING.md", "performance verification and regression checks"}}},
	{subs: []string{"execution", "finish", "complete", "workflow"}, docs: []routeDocSpec{
		{"AGENT_WORKFLOW.md", "agent start/work/verify/finish procedure"},
		{"TESTING.md", "verification evidence before completion"}}},
	{subs: []string{"commit", "push", "커밋", "푸시"}, tokens: []string{"pr"}, docs: []routeDocSpec{
		{"COMMIT_POLICY.md", "commit message, staging, and verification policy"},
		{"TESTING.md", "checks to run before commit/PR"},
		{"CAUTIONS.md", "project-specific commit risks"},
		{"AGENT_WORKFLOW.md", "finish procedure: verification results and skipped checks to report with the commit or PR/MR"}}},
	{subs: []string{"openapi", "swagger", "endpoint", "controller", "dto", "api doc", "api spec", "엔드포인트"}, docs: []routeDocSpec{
		{"OPEN_API_SPEC.md", "project-specific OpenAPI/Swagger static and agent review prompt"},
		{"TESTING.md", "API documentation check commands and static-vs-agent boundary"},
		{"AGENT_WORKFLOW.md", "verification workflow"},
		{"CAUTIONS.md", "known API documentation risks"}}},
	{subs: []string{"test", "testing", "테스트"}, tokens: []string{"ci"}, weakSubs: []string{"spec", "verify", "검증"}, docs: []routeDocSpec{
		{"TESTING.md", "well/poorly structured test guidance plus test/build/lint command candidates"},
		{"TECH_STACK.md", "toolchain evidence"},
		{"AGENT_WORKFLOW.md", "verification workflow"},
		{"CAUTIONS.md", "known verification risks"}}},
	{subs: []string{"ux", "style", "styling", "css", "typography", "palette", "theme", "color", "accessibility", "a11y", "animation", "motion", "redesign", "디자인", "화면", "스타일"}, docs: []routeDocSpec{
		{"DESIGN.md", "client design system: palette, typography, spacing, motion, accessibility, component states"},
		{"CONVENTIONS.md", "styling and component conventions"}}},
	{subs: []string{"architecture", "refactor", "구조", "아키텍처", "리팩터", "경계", "레이어", "컴포넌트", "모듈", "어댑터", "책임"}, tokens: []string{"design", "boundary", "boundaries", "layer", "layers", "component", "components", "module", "modules", "adapter", "adapters"}, docs: []routeDocSpec{
		{"ARCHITECTURE.md", "system structure and boundaries"},
		{"DESIGN.md", "client design system contract when present"},
		{"ADR.md", "past structure decisions and rejected alternatives"},
		{"CONSTITUTION.md", "decision priority and invariants"},
		{"CONVENTIONS.md", "editing and structure conventions"}}},
	{subs: []string{"dependency", "package", "upgrade", "stack", "의존성", "업그레이드", "라이브러리"}, docs: []routeDocSpec{
		{"TECH_STACK.md", "detected stack and package manager evidence"},
		{"CONVENTIONS.md", "dependency addition rules"},
		{"TESTING.md", "test design rules and checks after dependency changes"}}},
	{subs: []string{"deploy", "env", "operate", "install", "runtime", "배포", "설치", "운영", "런타임"}, weakSubs: []string{"run", "local"}, docs: []routeDocSpec{
		{"OPERATIONS.md", "local development, environment, and deployment guidance"},
		{"TECH_STACK.md", "toolchain evidence"},
		{"CAUTIONS.md", "operational risks"}}},
}

func containsAny(task string, subs []string) bool {
	for _, s := range subs {
		if strings.Contains(task, s) {
			return true
		}
	}
	return false
}

func hasAnyToken(tokens map[string]bool, names []string) bool {
	for _, n := range names {
		if tokens[n] {
			return true
		}
	}
	return false
}

func (r routeRule) matches(task string, tokens map[string]bool, matched bool) bool {
	if containsAny(task, r.subs) || hasAnyToken(tokens, r.tokens) {
		return true
	}
	return !matched && containsAny(task, r.weakSubs)
}

// routeTokens splits the task into letter/digit words. A Hangul run right after
// a Latin or digit run starts a new token, so a Korean particle attached to an
// English word ("adapter로", "pr을") still leaves the English word whole.
func routeTokens(task string) map[string]bool {
	tokens := make(map[string]bool)
	for field := range strings.FieldsFuncSeq(task, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		start, prevHangul := 0, true
		for i, r := range field {
			isHangul := unicode.Is(unicode.Hangul, r)
			if isHangul && !prevHangul {
				tokens[field[start:i]] = true
				start = i
			}
			prevHangul = isHangul
		}
		tokens[field[start:]] = true
	}
	return tokens
}

// RouteDocsForTask expects the lowercased task selected by the caller.
func RouteDocsForTask(task string) []RouteDoc {
	tokens := routeTokens(task)
	result := []RouteDoc{{"AGENTS.md", "repo-level agent entrypoint and document router"}}
	seen := map[string]bool{"AGENTS.md": true}
	appendDocs := func(specs []routeDocSpec) {
		for _, spec := range specs {
			rel := filepath.ToSlash(filepath.Join(ProjectDocsDir, spec.name))
			if !seen[rel] {
				result = append(result, RouteDoc{rel, spec.reason})
				seen[rel] = true
			}
		}
	}
	if containsAny(task, vcsRouteSubs) || hasAnyToken(tokens, []string{"pr", "mr"}) {
		appendDocs([]routeDocSpec{{"VCS.md", "verified VCS provider capabilities, exact request recipes, and CLI fallbacks"}})
	}
	matched := false
	for _, rule := range routeRules {
		if rule.matches(task, tokens, matched) {
			matched = true
			appendDocs(rule.docs)
		}
	}
	if !matched {
		appendDocs(defaultRouteDocs)
	}
	return result
}
