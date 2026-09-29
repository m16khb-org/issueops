package nativehost

import (
	"path/filepath"
	"strings"
)

// ExecutableMatchesHost checks only the caller-selected launcher identity. A
// terminal receiver still has to correlate the observed live process exactly;
// this name check cannot prove that a wrapper's descendant owns a session.
func ExecutableMatchesHost(host, executable string) bool {
	normalized := strings.ToLower(filepath.ToSlash(strings.TrimSpace(executable)))
	base := strings.TrimSuffix(filepath.Base(normalized), ".exe")
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "codex":
		return base == "codex"
	case "claude":
		return base == "claude" || strings.Contains(normalized, "/claude/versions/")
	case "omo":
		return base == "omo"
	default:
		return false
	}
}
