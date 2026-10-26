package remote

import (
	"fmt"
	"strings"
	"unicode"

	"issueops/internal/domain/secretdetection"
)

// ValidateComment refuses a comment body that must not reach the provider:
// empty, without Hangul (remote text is Korean by ADR 2026-09-24), over the
// provider limit, or carrying a secret-like value.
func ValidateComment(provider, body string) error {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return fmt.Errorf("comment body is empty; pass --body TEXT or --body-file PATH")
	}
	if !strings.ContainsFunc(trimmed, func(r rune) bool { return unicode.Is(unicode.Hangul, r) }) {
		return fmt.Errorf("comment body has no Korean text; remote issue and PR/MR text is written in Korean")
	}
	if err := ValidateProviderBodyLength(provider, "comment", body); err != nil {
		return err
	}
	if match, found := secretdetection.Find(body); found {
		return fmt.Errorf("comment body contains a secret-like value (%s on line %d); remove it before posting", match.Kind, match.Line)
	}
	return nil
}
