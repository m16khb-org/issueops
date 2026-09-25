package statepath

import (
	"fmt"
	"regexp"
	"strings"
)

var keyRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func NormalizeKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("state key is required")
	}
	if strings.Contains(key, "..") || strings.ContainsAny(key, `/\`) || !keyRe.MatchString(key) {
		return "", fmt.Errorf("invalid state key %q; use [A-Za-z0-9._-] without path separators or '..', max 128 chars", key)
	}
	return key, nil
}
