package issueopsintent

import (
	"strings"
	"unicode"

	"issueops/internal/domain/policy"
)

func MateriallyDifferentIntent(rawRequest, interpretedIntent string) bool {
	rawTokens := intentTokenSet(rawRequest)
	interpretedTokens := intentTokenSet(interpretedIntent)
	if len(rawTokens) < 4 || len(interpretedTokens) < 4 {
		return true
	}
	shared := 0
	for token := range rawTokens {
		if interpretedTokens[token] {
			shared++
		}
	}
	union := len(rawTokens) + len(interpretedTokens) - shared
	return union == 0 || float64(shared)/float64(union) < 0.85
}

func intentTokenSet(text string) map[string]bool {
	out := map[string]bool{}
	for _, token := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if token == "" || intentStopWord(token) {
			continue
		}
		out[token] = true
	}
	return out
}

func intentStopWord(token string) bool {
	switch token {
	case "a", "an", "the", "please", "좀", "해주세요":
		return true
	default:
		return false
	}
}

func CleanTextValues(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || strings.Contains(value, "\x00") || seen[value] {
			continue
		}
		value = policy.RedactFreeform(value)
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
