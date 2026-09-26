package projectdocs

import (
	"path/filepath"

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
	return projectdocsapp.Read(root, rel, path, revisionFileEffects{})
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
