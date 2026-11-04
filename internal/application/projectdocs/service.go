package projectdocs

import (
	docscontract "issueops/internal/contract/projectdocs"
	docdomain "issueops/internal/domain/projectdoc"
)

// Service keeps repository observation and file effects local to one caller.
type Service struct {
	NormalizeRoot func(string) (string, error)
	Revision      RevisionEffects
	Routing       RouteEffects
	Appending     AppendEffects
}

func (service Service) Read(root, rel string) (docscontract.ReadResult, error) {
	root, err := service.NormalizeRoot(root)
	if err != nil {
		return docscontract.ReadResult{}, err
	}
	rel, err = docdomain.NormalizeRelPath(rel)
	if err != nil {
		return docscontract.ReadResult{}, err
	}
	return Read(root, rel, service.Routing.Path(root, rel), service.Revision)
}

func (service Service) Revise(request docscontract.ReviseRequest) (docscontract.ReviseResult, error) {
	root, err := service.NormalizeRoot(request.RepoRoot)
	if err != nil {
		return docscontract.ReviseResult{}, err
	}
	rel, err := docdomain.NormalizeRelPath(request.RelPath)
	if err != nil {
		return docscontract.ReviseResult{}, err
	}
	return Revise(request, root, rel, service.Routing.Path(root, rel), service.Revision)
}

func (service Service) Route(root, task string) (docscontract.RouteResult, error) {
	root, err := service.NormalizeRoot(root)
	if err != nil {
		return docscontract.RouteResult{}, err
	}
	return Route(root, task, service.Routing), nil
}

func (service Service) Append(request docscontract.AppendRequest) (docscontract.AppendResult, error) {
	root, err := service.NormalizeRoot(request.RepoRoot)
	if err != nil {
		return docscontract.AppendResult{}, err
	}
	return Append(root, request, service.Appending)
}
