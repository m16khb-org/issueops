package guard

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	guardcontract "issueops/internal/contract/guard"
)

func reuseFinding(rel string, line int, symbol string, existing map[string][]string) (guardcontract.Finding, bool) {
	key := NormalizeSymbol(symbol)
	if key == "" || len(existing[key]) == 0 {
		return guardcontract.Finding{}, false
	}
	return guardcontract.Finding{
		Severity: "review",
		Rule:     "reuse-before-new",
		File:     rel,
		Line:     line,
		Message:  "New symbol resembles existing repository code; confirm reuse or record why a new implementation is necessary.",
		Evidence: symbol,
		Suggestions: []string{
			fmt.Sprintf("Review existing candidates: %s", strings.Join(existing[key], ", ")),
			fmt.Sprintf("Search repo for similar helpers: rg %q .", symbol),
		},
	}, true
}

// NormalizeSymbol turns a symbol name into a comparison key by splitting it into lowercase tokens,
// dropping plural suffixes and tokens of two characters or fewer, and sorting them. Two symbols with
// the same key resemble each other. It returns "" when no token remains.
func NormalizeSymbol(symbol string) string {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return ""
	}
	filtered := []string{}
	for _, token := range splitSymbolTokens(symbol) {
		token = strings.TrimSuffix(token, "s")
		if len(token) > 2 {
			filtered = append(filtered, token)
		}
	}
	if len(filtered) == 0 {
		return ""
	}
	sort.Strings(filtered)
	b, _ := json.Marshal(filtered)
	return string(b)
}

// splitSymbolTokens는 snake/kebab/camel 경계에서 심볼을 소문자 토큰으로 자른다.
func splitSymbolTokens(symbol string) []string {
	var tokens []string
	var current strings.Builder
	var previousLower bool
	for _, r := range symbol {
		if r == '_' || r == '-' {
			if current.Len() > 0 {
				tokens = append(tokens, strings.ToLower(current.String()))
				current.Reset()
			}
			previousLower = false
			continue
		}
		isUpper := r >= 'A' && r <= 'Z'
		if isUpper && previousLower && current.Len() > 0 {
			tokens = append(tokens, strings.ToLower(current.String()))
			current.Reset()
		}
		current.WriteRune(r)
		previousLower = r >= 'a' && r <= 'z'
	}
	if current.Len() > 0 {
		tokens = append(tokens, strings.ToLower(current.String()))
	}
	return tokens
}
