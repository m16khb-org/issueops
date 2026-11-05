// Package guard is the adapter that feeds the guard check with repository
// facts: which files to inspect, which symbols already exist, and file
// contents from the working tree or the git index. The verdict itself is the
// guard domain's job.
package guard

import (
	guardcontract "issueops/internal/contract/guard"
	"path/filepath"
)

// Source reads guard inputs from the filesystem and git. BaseDir, when set,
// anchors relative root paths.
type Source struct{ BaseDir string }

// ResolveRoot returns the absolute repository root for path, resolving a
// relative path against BaseDir and falling back to the current directory when
// path cannot be made absolute.
func (source Source) ResolveRoot(path string) string {
	if source.BaseDir != "" && !filepath.IsAbs(path) {
		path = filepath.Join(source.BaseDir, path)
	}
	root := absOrOriginal(path)
	if root == "" {
		root = absOrOriginal(".")
	}
	return root
}

// TargetFiles returns the repo-relative files a guard check covers: the
// request's explicit files, else every file under root when request.All is set,
// else the files staged in git (added, copied, modified, or renamed).
func (Source) TargetFiles(root string, request guardcontract.CheckRequest) []string {
	return guardTargetFiles(root, request)
}

// ExistingSymbols scans the source files under root, skipping the files in
// files themselves, and returns each symbol name that is already defined there
// mapped to the sorted, de-duplicated files that define it.
func (Source) ExistingSymbols(root string, files []string) map[string][]string {
	return guardExistingSymbols(root, files)
}

// ReadFile returns the content of root-relative rel and whether it could be
// read. With staged set it prefers the git index version and falls back to the
// working tree.
func (Source) ReadFile(root, rel string, staged bool) (string, bool) {
	return guardReadFile(root, rel, staged)
}
