package projectbootstrap

import contract "issueops/internal/contract/projectbootstrap"

type Service struct {
	NormalizeRoot func(string) (string, error)
	Effects       Effects
}

func (service Service) Run(request contract.ProjectDocsBootstrapRequest) (contract.ProjectDocsBootstrapResult, error) {
	root, err := service.NormalizeRoot(request.RepoRoot)
	if err != nil {
		return contract.ProjectDocsBootstrapResult{}, err
	}
	request.RepoRoot = root
	return Bootstrap(request, service.Effects)
}
