// Package augmentation is the outbound adapter behind the self-augment planner: it reads
// GENIUS_THINK.md, inspects git state, and derives repository signals by scanning docs and
// Go sources on the local filesystem.
package augmentation

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"issueops/internal/adapter/outbound/gitbin"
	port "issueops/internal/port/selfaugment"
)

// Repository is the filesystem and git backed implementation of the self-augment plan
// repository port. ListDocs enumerates the documentation files under a root.
type Repository struct{ ListDocs func(string) []string }

var _ port.PlanRepository = Repository{}

// ReadGeniusThink reads GENIUS_THINK.md under root and returns its path and content.
// The path is returned even when reading fails, together with the read error.
func (Repository) ReadGeniusThink(root string) (string, string, error) {
	path := filepath.Join(root, "GENIUS_THINK.md")
	content, err := os.ReadFile(path)
	return path, string(content), err
}

// HasImplementationDelta reports whether the git working tree at root has any uncommitted
// change. It returns false when git fails, so a non-repository counts as no delta.
func (Repository) HasImplementationDelta(root string) bool {
	cmd := exec.Command(gitbin.Path(), "status", "--porcelain")
	cmd.Dir = root
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}
