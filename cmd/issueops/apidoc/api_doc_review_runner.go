package apidoc

import "path/filepath"

func resolveAPIDocReviewResultPath(repo, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(repo, path)
}
