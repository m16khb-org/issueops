package riskqa

import (
	"path/filepath"
	"sort"
)

func NormalizePaths(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = filepath.ToSlash(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
