package issueops

import (
	"io/fs"
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

// ProjectDocsPresent reports AGENTS.md or any markdown under .issueops/ other
// than cycle material (issues/, gates/). An empty root reports true so the
// caller keeps requiring a reviewed doc it cannot disprove.
func (ReviewDocumentPaths) ProjectDocsPresent(root string) bool {
	if strings.TrimSpace(root) == "" {
		return true
	}
	if info, err := os.Stat(filepath.Join(root, "AGENTS.md")); err == nil && !info.IsDir() {
		return true
	}
	docs := filepath.Join(root, ".issueops")
	found := false
	_ = filepath.WalkDir(docs, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			if rel, _ := filepath.Rel(docs, path); rel == "issues" || rel == "gates" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".md") {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}

func (ReviewDocumentPaths) FileExists(root, relative string) bool {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative)))
	return err == nil && !info.IsDir()
}
