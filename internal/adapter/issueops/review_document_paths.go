package issueops

import (
	"issueops/internal/adapter/issueops/readinesspaths"
	model "issueops/internal/contract/issueops"
	"os"
	"path/filepath"
	"strings"
)

type ReviewDocumentPaths struct{}

func (ReviewDocumentPaths) Root(record model.IssueOpsRecord) string {
	return readinesspaths.StrictGitRoot(record)
}

func (ReviewDocumentPaths) RelativePath(root, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		if root == "" {
			return ""
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return ""
		}
		path = rel
	}
	path = filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if path == "." || path == ".." || strings.HasPrefix(path, "../") {
		return ""
	}
	return path
}

func (ReviewDocumentPaths) FileExists(root, relative string) bool {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative)))
	return err == nil && !info.IsDir()
}
