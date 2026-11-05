package providerutil

import "strings"

// SameStrings reports whether want and got hold the same values once each is
// trimmed, ignoring order, duplicates, and blank entries.
func SameStrings(want, got []string) bool {
	return len(MissingStrings(want, got)) == 0 && len(MissingStrings(got, want)) == 0
}

// MissingStrings returns the trimmed, non-blank values of want that are absent from
// got, in want order. Values of got are trimmed before comparison.
func MissingStrings(want, got []string) []string {
	gotSet := make(map[string]bool, len(got))
	for _, value := range got {
		gotSet[strings.TrimSpace(value)] = true
	}
	var missing []string
	for _, value := range want {
		value = strings.TrimSpace(value)
		if value != "" && !gotSet[value] {
			missing = append(missing, value)
		}
	}
	return missing
}

// FirstNonEmpty returns the first value that is non-blank after trimming, already
// trimmed, or "" when every value is blank.
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
