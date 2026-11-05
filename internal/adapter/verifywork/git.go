// Package verifywork is the adapter that supplies working-tree state (git
// status) to the verify-work application service.
package verifywork

import (
	"os/exec"

	"issueops/internal/adapter/outbound/gitbin"
)

// GitStatus returns the `git status --short` output for the repository at
// root, using the resolved git binary.
func GitStatus(root string) (string, error) {
	output, err := exec.Command(gitbin.Path(), "-C", root, "status", "--short").Output()
	return string(output), err
}
