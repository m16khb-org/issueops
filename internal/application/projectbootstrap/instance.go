// Package projectbootstrap is the application service that creates or
// refreshes a repository's AGENTS.md and .issueops project documents from
// repository evidence.
package projectbootstrap

import contract "issueops/internal/contract/projectbootstrap"

// Service bootstraps the project docs of a repository. NormalizeRoot resolves
// the repository root and Effects supplies the filesystem and repository reads.
type Service struct {
	NormalizeRoot func(string) (string, error)
	Effects       effects
}

// Run normalizes request.RepoRoot with NormalizeRoot and bootstraps the project
// docs for it, returning the planned or written files and warnings. It returns
// the NormalizeRoot error unchanged.
func (service Service) Run(request contract.ProjectDocsBootstrapRequest) (contract.ProjectDocsBootstrapResult, error) {
	root, err := service.NormalizeRoot(request.RepoRoot)
	if err != nil {
		return contract.ProjectDocsBootstrapResult{}, err
	}
	request.RepoRoot = root
	return bootstrap(request, service.Effects)
}
