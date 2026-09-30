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
func (e IssueCreationEnvironment) Resolve(name string) (application.IssueCreateInvoker, error) {
	resolved, err := e.ResolveProvider(name)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, req port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
		return CreateRemoteIssueContext(ctx, req, resolved)
	}, nil
}

var _ application.IssueCreationEnvironment = IssueCreationEnvironment{}
