package issueops

import (
	"context"

	application "issueops/internal/application/issueopsremote"
	"issueops/internal/port"
)

type IssueCreationEnvironment struct {
	ResolveProvider func(string) (port.IssueProvider, error)
}

func (e IssueCreationEnvironment) InferProvider(repo string) (string, error) {
	return InferProviderFromRepoRemotes(repo)
}
func (e IssueCreationEnvironment) ProjectAuthority(repo, provider string) (string, error) {
	return ResolveProviderProjectAuthority(repo, provider)
}
func (e IssueCreationEnvironment) Resolve(name string) (application.IssueCreationProvider, error) {
	resolved, err := e.ResolveProvider(name)
	if err != nil {
		return nil, err
	}
	return issueCreationProvider{provider: resolved}, nil
}

type issueCreationProvider struct{ provider port.IssueProvider }

func (p issueCreationProvider) Create(ctx context.Context, req port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
	if !req.Confirm {
		ctx = context.Background()
	}
	return CreateRemoteIssueContext(ctx, req, p.provider)
}

var _ application.IssueCreationEnvironment = IssueCreationEnvironment{}
