// Package stringlist provides small helpers for string slices.
package stringlist

import "sort"

// UniqueSorted returns the non-empty values of in without duplicates, sorted
// ascending. It never returns nil.
func UniqueSorted(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range in {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
