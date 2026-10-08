package statepath

import (
	"fmt"
	"strings"
)

// validKey matches ^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$ by hand: the counted
// repetition compiles to a large regexp program at every process start.
func validKey(key string) bool {
	if len(key) == 0 || len(key) > 128 || !isAlnum(key[0]) {
		return false
	}
	for i := range len(key) {
		if c := key[i]; !isAlnum(c) && c != '.' && c != '_' && c != '-' {
			return false
		}
	}
	return true
}

func isAlnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func NormalizeKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("state key is required")
	}
	if strings.Contains(key, "..") || strings.ContainsAny(key, `/\`) || !validKey(key) {
		return "", fmt.Errorf("invalid state key %q; use [A-Za-z0-9._-] without path separators or '..', max 128 chars", key)
	}
	return key, nil
}
