// Package riskqa holds the contract types of risk based QA planning: the
// verification tier plan and the tree scope it was derived from.
package riskqa

import (
	"path/filepath"
	"sort"
)

// NormalizePaths returns values converted to slash form, with empty and
// duplicate entries removed, in sorted order. It never returns nil.
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
