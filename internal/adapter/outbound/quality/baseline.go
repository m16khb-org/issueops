// Package quality is the outbound adapter that measures repository quality
// signals for the quality report: Go test coverage (with an exact-fingerprint
// cache), code signal-to-noise ratio, branch counts per function, audit items,
// and pioneer skill coverage.
package quality

import (
	"fmt"
	"path/filepath"
)

// CanonicalRepository returns root as a cleaned absolute path with symlinks
// resolved, so one repository maps to one SNR baseline key.
func CanonicalRepository(root string) (string, error) {
	repository, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve quality SNR repository: %w", err)
	}
	repository, err = filepath.EvalSymlinks(repository)
	if err != nil {
		return "", fmt.Errorf("resolve quality SNR repository symlinks: %w", err)
	}
	return filepath.Clean(repository), nil
}
