package projectdocs

import (
	"time"

	projectdocscontract "issueops/internal/contract/projectdocs"
	projectdocdomain "issueops/internal/domain/projectdoc"
)

// ReadEffects is the file and clock access Read needs.
type ReadEffects interface {
	Read(path string) (string, bool, error)
	Now() time.Time
}

// Read returns the content and SHA-256 of the document at path. A missing document is not an error:
// the result has Exists false and a warning pointing at project bootstrap.
func Read(root, rel, path string, effects ReadEffects) (projectdocscontract.ReadResult, error) {
	result := projectdocscontract.ReadResult{
		OK: true, Kind: "project_docs_read", RepoRoot: root, RelPath: rel, Path: path,
		GeneratedAt: effects.Now().UTC().Format(time.RFC3339),
	}
	content, exists, err := effects.Read(path)
	if err != nil {
		return projectdocscontract.ReadResult{}, err
	}
	if !exists {
		result.Warnings = []string{"document_missing: run project_docs_bootstrap_plan or issueops project bootstrap first"}
		return result, nil
	}
	result.Exists = true
	result.Content = content
	result.SHA256 = projectdocdomain.SHA256Hex(content)
	return result, nil
}
