package guard

import (
	guardapp "issueops/internal/application/guard"
	guardcontract "issueops/internal/contract/guard"
)

func GuardCheck(req guardcontract.GuardCheckRequest) guardcontract.GuardCheckResult {
	return (guardapp.Service{Source: guardSource{}}).Check(req)
}

type guardSource struct{}

func (guardSource) ResolveRoot(path string) string {
	root := absOrOriginal(path)
	if root == "" {
		root = absOrOriginal(".")
	}
	return root
}

func (guardSource) TargetFiles(root string, request guardcontract.GuardCheckRequest) []string {
	return guardTargetFiles(root, request)
}
func (guardSource) ExistingSymbols(root string, files []string) map[string][]string {
	return guardExistingSymbols(root, files)
}
func (guardSource) ReadFile(root, rel string, staged bool) (string, bool) {
	return guardReadFile(root, rel, staged)
}
