package guard

import (
	guardapp "issueops/internal/application/guard"
	guardcontract "issueops/internal/contract/guard"
)

func GuardCheck(req guardcontract.GuardCheckRequest) guardcontract.GuardCheckResult {
	return (guardapp.Service{Source: Source{}}).Check(req)
}

type Source struct{}

func (Source) ResolveRoot(path string) string {
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
