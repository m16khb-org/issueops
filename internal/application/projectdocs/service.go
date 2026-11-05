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

// Read returns the document at rel under the normalized root; see the Read function.
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

// Revise replaces one project document after validating the request and its expected hash.
// Without confirmation it only plans the change.
func (service Service) Revise(request docscontract.ReviseRequest) (docscontract.ReviseResult, error) {
	root, err := service.NormalizeRoot(request.RepoRoot)
	if err != nil {
		return docscontract.ReviseResult{}, err
	}
	rel, err := docdomain.NormalizeRelPath(request.RelPath)
	if err != nil {
		return docscontract.ReviseResult{}, err
	}
	return revise(request, root, rel, service.Routing.Path(root, rel), service.Revision)
}

// Route returns the project documents and dated records a task should read before work starts.
// It only reads; missing docs are reported as warnings.
func (service Service) Route(root, task string) (docscontract.RouteResult, error) {
	root, err := service.NormalizeRoot(root)
	if err != nil {
		return docscontract.RouteResult{}, err
	}
	return route(root, task, service.Routing), nil
}

// Append writes one new dated caution or ADR record and links it from the family overview.
func (service Service) Append(request docscontract.AppendRequest) (docscontract.AppendResult, error) {
	root, err := service.NormalizeRoot(request.RepoRoot)
	if err != nil {
		return docscontract.AppendResult{}, err
	}
	return Append(root, request, service.Appending)
}
