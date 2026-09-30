package commitsuggest

import "strings"

func NeedsSuggestion(diff string) bool { return strings.TrimSpace(diff) != "" }
