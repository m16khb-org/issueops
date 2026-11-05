// Package docs defines the outbound port the document index application uses
// to observe the repository.
package docs

import docscontract "issueops/internal/contract/docs"

// Observer is the outbound port that reads the repository for the document
// index: candidate files under roots, the Git tracked set (and whether Git was
// available), module directories, and per-document metadata.
type Observer interface {
	Candidates(root string, roots []string) []docscontract.Candidate
	TrackedPaths(root string) (map[string]bool, bool)
	ModuleDirectories(root string) []string
	ReadDocument(root, path string) (docscontract.DocIndexInfo, bool)
}
