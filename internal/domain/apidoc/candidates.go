// Package apidoc is the domain layer for API documentation checks: it decides which files are API candidates and
// statically checks NestJS controllers and DTOs for missing Swagger/OpenAPI documentation. It works on text
// already read by the caller and does no I/O.
package apidoc

import (
	contract "issueops/internal/contract/apidoc"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var candidateRe = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`(?i)(controller|dto|route|router|handler|endpoint|openapi|swagger|api|schema|proto)`)
})

// IsCandidate reports whether file may hold API surface worth documenting: its path names a controller,
// DTO, route, handler, endpoint, OpenAPI, schema, or similar. Markdown, text, package.json, and lock files never qualify.
func IsCandidate(file string) bool {
	base := filepath.Base(file)
	ext := strings.ToLower(filepath.Ext(base))
	if ext == ".md" || ext == ".txt" {
		return false
	}
	if base == "package.json" || strings.HasSuffix(base, "lock") {
		return false
	}
	return candidateRe().MatchString(file)
}

// ValidReviewVerdict reports whether verdict is one of the accepted review verdicts, "pass" or "fail".
func ValidReviewVerdict(verdict string) bool { return verdict == "pass" || verdict == "fail" }

// StaticKinds classifies a TypeScript file by name for the static checks: controller covers controller, handler,
// route, and router files; dto covers dto and schema files. Non-.ts files are neither.
func StaticKinds(file string) (controller, dto bool) {
	if !strings.HasSuffix(file, ".ts") {
		return false, false
	}
	lower := strings.ToLower(file)
	controller = strings.Contains(lower, "controller") || strings.Contains(lower, "handler") || strings.Contains(lower, "route") || strings.Contains(lower, "router")
	dto = strings.Contains(lower, "dto") || strings.Contains(lower, "schema")
	return controller, dto
}

// SortViolations orders violations in place by file, then line, then code, so output is deterministic.
func SortViolations(violations []contract.Violation) {
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].File != violations[j].File {
			return violations[i].File < violations[j].File
		}
		if violations[i].Line != violations[j].Line {
			return violations[i].Line < violations[j].Line
		}
		return violations[i].Code < violations[j].Code
	})
}
