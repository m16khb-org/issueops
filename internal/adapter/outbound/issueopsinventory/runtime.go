// Package issueopsinventory is the outbound adapter that backs the IssueOps cycle
// inventory: it lists and scans stored cycle records, and supplies the clock and
// repository path normalization the inventory needs.
package issueopsinventory

import (
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"issueops/internal/adapter/outbound/gitbin"
	"issueops/internal/domain/repoidentity"
)

// SystemClock is the inventory clock backed by the system time.
type SystemClock struct{}

// Now returns the current time.
func (SystemClock) Now() time.Time { return time.Now() }

// CleanPath is the inventory path port: it normalizes and canonicalizes
// repository paths.
type CleanPath struct{}

// Canonical is the absolute, cleaned, symlink-resolved form of path, so
// /tmp/x and /private/tmp/x (macOS) or a symlinked workspace compare equal.
// It is for comparisons only: stored and printed identities stay lexical
// (Normalize). A path that does not exist keeps its cleaned form.
func Canonical(path string) string {
	clean := lexical(path)
	if clean == "" {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(clean); err == nil {
		clean = resolved
	}
	return clean
}

// Canonical is Canonical as a method of the inventory path port.
func (CleanPath) Canonical(path string) string { return Canonical(path) }

func lexical(path string) string {
	clean := strings.TrimSpace(path)
	if clean == "" {
		return ""
	}
	if !filepath.IsAbs(clean) {
		if absolute, err := filepath.Abs(clean); err == nil {
			clean = absolute
		}
	}
	return filepath.Clean(clean)
}

// Normalize returns the absolute, cleaned identity of path. For a path inside a
// git worktree it returns the source repository root so every worktree of one
// repository shares an identity; outside git it returns the cleaned path. An
// empty path yields an empty string.
func (CleanPath) Normalize(path string) string {
	clean := lexical(path)
	if clean == "" {
		return ""
	}
	command := exec.Command(gitbin.Path(), "rev-parse", "--path-format=relative", "--git-common-dir")
	command.Dir = clean
	commonDir, err := command.Output()
	if err != nil {
		return clean
	}
	return repoidentity.SourceRoot(clean, string(commonDir))
}
