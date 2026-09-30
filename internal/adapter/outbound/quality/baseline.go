package quality

import (
	"fmt"
	"path/filepath"
)

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
