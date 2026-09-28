package issueops

import (
	"fmt"
	"path/filepath"
	"strings"
)

func cleanupStrictSamePath(left, right string) (bool, error) {
	canonical := func(path string) (string, error) {
		path = strings.TrimSpace(path)
		if path == "" {
			return "", fmt.Errorf("path is empty")
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		resolved, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			return "", err
		}
		return filepath.Clean(resolved), nil
	}
	leftPath, err := canonical(left)
	if err != nil {
		return false, err
	}
	rightPath, err := canonical(right)
	if err != nil {
		return false, err
	}
	return leftPath == rightPath, nil
}
