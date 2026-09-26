package apidoc

import (
	"sort"
	"strings"
)

func StaticKinds(file string) (controller, dto bool) {
	if !strings.HasSuffix(file, ".ts") {
		return false, false
	}
	lower := strings.ToLower(file)
	controller = strings.Contains(lower, "controller") || strings.Contains(lower, "handler") || strings.Contains(lower, "route") || strings.Contains(lower, "router")
	dto = strings.Contains(lower, "dto") || strings.Contains(lower, "schema")
	return controller, dto
}

func SortViolations(violations []Violation) {
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
