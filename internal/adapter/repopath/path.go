// Package repopath normalizes repository root paths supplied by callers.
package repopath

import (
	"fmt"
	"os"
	"path/filepath"
)

// NormalizeRoot returns root as an absolute path, defaulting an empty root to
// the current directory. It fails when the path does not exist or is not a
// directory.
func NormalizeRoot(root string) (string, error) {
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("repo root is not a directory: %s", abs)
	}
	return abs, nil
}
