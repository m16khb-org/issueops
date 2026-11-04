// Package commitsuggest provides the git-backed effects for commit message
// suggestions: root normalization and diff reading. It is an adapter-layer
// package.
package commitsuggest

import (
	"os/exec"

	"issueops/internal/adapter/outbound/gitbin"
)

// Effects runs the commit suggestion side effects. Normalize resolves the
// repository root path.
type Effects struct{ Normalize func(string) (string, error) }

// NormalizeRoot resolves root through the injected Normalize function.
func (effects Effects) NormalizeRoot(root string) (string, error) { return effects.Normalize(root) }

// Diff returns the output of `git diff` in root, restricted to staged changes
// when staged is true.
func (Effects) Diff(root string, staged bool) (string, error) {
	args := []string{"-C", root, "diff"}
	if staged {
		args = append(args, "--cached")
	}
	output, err := exec.Command(gitbin.Path(), args...).Output()
	return string(output), err
}
