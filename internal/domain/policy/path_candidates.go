package policy

import (
	"path/filepath"
	"strings"
)

func PathCandidates(arg string) []string {
	arg = strings.TrimSpace(arg)
	if arg == "" || looksLikeRemoteOrURL(arg) {
		return nil
	}
	if strings.HasPrefix(arg, "-") {
		if key, value, ok := strings.Cut(arg, "="); ok && strings.TrimSpace(key) != "" && argLooksPathLike(value) {
			return []string{value}
		}
		return nil
	}
	if !argLooksPathLike(arg) {
		return nil
	}
	return []string{arg}
}

func argLooksPathLike(arg string) bool {
	arg = strings.TrimSpace(arg)
	if arg == "" || looksLikeRemoteOrURL(arg) {
		return false
	}
	if arg == "~" || strings.HasPrefix(arg, "~/") || strings.HasPrefix(arg, "~"+string(filepath.Separator)) {
		return true
	}
	if filepath.IsAbs(arg) || arg == "." || arg == ".." {
		return true
	}
	slashArg := filepath.ToSlash(arg)
	return strings.HasPrefix(slashArg, "./") || strings.HasPrefix(slashArg, "../") || strings.Contains(slashArg, "/")
}

func looksLikeRemoteOrURL(arg string) bool {
	lower := strings.ToLower(arg)
	if strings.Contains(lower, "://") {
		return true
	}
	if at := strings.Index(arg, "@"); at >= 0 {
		return strings.Contains(arg[at+1:], ":")
	}
	return false
}
