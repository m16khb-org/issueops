package projectbootstrap

import projectbootstrapapp "issueops/internal/application/projectbootstrap"

func BootstrapProjectDocs(request ProjectDocsBootstrapRequest) (ProjectDocsBootstrapResult, error) {
	root, err := NormalizeRepoRoot(request.RepoRoot)
	if err != nil {
		return ProjectDocsBootstrapResult{}, err
	}
	request.RepoRoot = root
	return projectbootstrapapp.Bootstrap(request, bootstrapEffects{})
}
