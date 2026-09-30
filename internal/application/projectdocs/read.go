package projectdocs

import (
	"time"

	projectdocscontract "issueops/internal/contract/projectdocs"
	projectdocdomain "issueops/internal/domain/projectdoc"
)

type ReadEffects interface {
	Read(path string) (string, bool, error)
	Now() time.Time
}

func Read(root, rel, path string, effects ReadEffects) (projectdocscontract.ProjectDocsReadResult, error) {
	result := projectdocscontract.ProjectDocsReadResult{
		OK: true, Kind: "project_docs_read", RepoRoot: root, RelPath: rel, Path: path,
		GeneratedAt: effects.Now().Format(time.RFC3339),
	}
	content, exists, err := effects.Read(path)
	if err != nil {
		return projectdocscontract.ProjectDocsReadResult{}, err
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
