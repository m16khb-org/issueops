// Package commitsuggest holds the domain rule deciding when a commit message
// suggestion is needed.
package commitsuggest

import "strings"

// NeedsSuggestion reports whether diff has any non-whitespace content worth
// suggesting a commit message for.
func NeedsSuggestion(diff string) bool { return strings.TrimSpace(diff) != "" }
