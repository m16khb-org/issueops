package issueops

import (
	"fmt"
	"strings"
)

func ValidateBranch(branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return nil
	}
	if reason := refNameViolation(branch); reason != "" {
		return fmt.Errorf("issueops branch contains invalid characters: %s (git branch names cannot %s)", branch, reason)
	}
	issueNumber, slug, ok := strings.Cut(branch, "-")
	if !ok || strings.TrimSpace(slug) == "" || !isDecimalString(issueNumber) {
		return fmt.Errorf("issueops branch must start with the issue number followed by a hyphen; use names like 123-fix-login-timeout or 456-refactor-issueops-gates")
	}
	return nil
}

// refNameViolation applies git check-ref-format --branch rules so a name git
// would refuse is rejected before any record or remote branch carries it.
func refNameViolation(branch string) string {
	for _, r := range branch {
		if r < 0x20 || r == 0x7f || r == ' ' {
			return "contain spaces or control characters"
		}
		if strings.ContainsRune("~^:?*[\\", r) {
			return "contain " + string(r)
		}
	}
	switch {
	case strings.Contains(branch, ".."):
		return "contain .."
	case strings.Contains(branch, "@{"):
		return "contain @{"
	case branch == "@":
		return "be @"
	case strings.HasPrefix(branch, "/"), strings.HasSuffix(branch, "/"), strings.Contains(branch, "//"):
		return "start or end with / or contain //"
	case strings.HasSuffix(branch, "."):
		return "end with ."
	}
	for component := range strings.SplitSeq(branch, "/") {
		if strings.HasPrefix(component, ".") {
			return "have a path component starting with ."
		}
		if strings.HasSuffix(component, ".lock") {
			return "have a path component ending with .lock"
		}
	}
	return ""
}

func isDecimalString(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
