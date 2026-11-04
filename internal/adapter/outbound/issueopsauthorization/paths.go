// Package issueopsauthorization provides the filesystem-backed path comparison
// used by IssueOps authorization decisions; it is an outbound adapter.
package issueopsauthorization

import (
	"path/filepath"
	"strings"
)

// CanonicalPaths compares paths by their canonical filesystem identity.
type CanonicalPaths struct{}

// Same reports whether left and right resolve to the same absolute path after
// symlink evaluation. It returns false if either path cannot be canonicalized.
func (CanonicalPaths) Same(left, right string) bool {
	left, err := canonicalPath(left)
	if err != nil {
		return false
	}
	right, err = canonicalPath(right)
	return err == nil && left == right
}

func canonicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return "", err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
		absolute = resolved
	}
	return filepath.Clean(absolute), nil
}
