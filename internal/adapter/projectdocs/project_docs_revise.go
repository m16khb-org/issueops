package projectdocs

import (
	"os"
	"path/filepath"
	"time"

	projectdocsapp "issueops/internal/application/projectdocs"
	projectdocscontract "issueops/internal/contract/projectdocs"
)

func ReadProjectDoc(repoRoot, relPath string) (projectdocscontract.ProjectDocsReadResult, error) {
	root, err := NormalizeRepoRoot(repoRoot)
	if err != nil {
		return projectdocscontract.ProjectDocsReadResult{}, err
	}
	rel, err := normalizeProjectDocRelPath(relPath)
	if err != nil {
		return projectdocscontract.ProjectDocsReadResult{}, err
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	result := projectdocscontract.ProjectDocsReadResult{
		OK:          true,
		Kind:        "project_docs_read",
		RepoRoot:    root,
		RelPath:     rel,
		Path:        path,
		GeneratedAt: time.Now().Format(time.RFC3339),
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		result.Exists = false
		result.Warnings = []string{"document_missing: run project_docs_bootstrap_plan or issueops project bootstrap first"}
		return result, nil
	}
	if err != nil {
		return projectdocscontract.ProjectDocsReadResult{}, err
	}
	result.Exists = true
	result.Content = string(b)
	result.SHA256 = sha256Hex(result.Content)
	return result, nil
}

func ReviseProjectDoc(req projectdocscontract.ProjectDocsReviseRequest) (projectdocscontract.ProjectDocsReviseResult, error) {
	root, err := NormalizeRepoRoot(req.RepoRoot)
	if err != nil {
		return projectdocscontract.ProjectDocsReviseResult{}, err
	}
	rel, err := normalizeProjectDocRelPath(req.RelPath)
	if err != nil {
		return projectdocscontract.ProjectDocsReviseResult{}, err
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	return projectdocsapp.Revise(req, root, rel, path, revisionFileEffects{})
}
