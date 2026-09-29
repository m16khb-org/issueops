package docs

import docscontract "issueops/internal/contract/docs"

type Observer interface {
	Candidates(root string, roots []string) []docscontract.Candidate
	TrackedPaths(root string) (map[string]bool, bool)
	ModuleDirectories(root string) []string
	ReadDocument(root, path string) (docscontract.DocIndexInfo, bool)
}
