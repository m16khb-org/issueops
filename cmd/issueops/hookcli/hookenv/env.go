// Package hookenv reads hook operator switches from environment variables.
package hookenv

import (
	"os"
	"strings"
)

// Bool reports whether environment variable name is set to a truthy value (1,
// true, yes, or on, case-insensitive, surrounding space ignored). Unset and any
// other value are false.
func Bool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
