package apidoc

import (
	contract "issueops/internal/contract/apidoc"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var candidateRe = regexp.MustCompile(`(?i)(controller|dto|route|router|handler|endpoint|openapi|swagger|api|schema|proto)`)

func IsCandidate(file string) bool {
	base := filepath.Base(file)
	ext := strings.ToLower(filepath.Ext(base))
	if ext == ".md" || ext == ".txt" {
		return false
	}
	if base == "package.json" || strings.HasSuffix(base, "lock") {
		return false
	}
	return candidateRe.MatchString(file)
}

func ValidReviewVerdict(verdict string) bool { return verdict == "pass" || verdict == "fail" }

func StaticKinds(file string) (controller, dto bool) {
	if !strings.HasSuffix(file, ".ts") {
		return false, false
	}
	lower := strings.ToLower(file)
	controller = strings.Contains(lower, "controller") || strings.Contains(lower, "handler") || strings.Contains(lower, "route") || strings.Contains(lower, "router")
	dto = strings.Contains(lower, "dto") || strings.Contains(lower, "schema")
	return controller, dto
}

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
