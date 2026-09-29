package guard

import (
	guardcontract "issueops/internal/contract/guard"
	"path/filepath"
)

type Source struct{ BaseDir string }

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

func (Source) TargetFiles(root string, request guardcontract.GuardCheckRequest) []string {
	return guardTargetFiles(root, request)
}
func (Source) ExistingSymbols(root string, files []string) map[string][]string {
	return guardExistingSymbols(root, files)
}
func (Source) ReadFile(root, rel string, staged bool) (string, bool) {
	return guardReadFile(root, rel, staged)
}
