package issueops

import (
	"context"
	"fmt"

	application "issueops/internal/application/issueopsremote"
	"issueops/internal/port"
)

// IssueCreationEnvironment supplies provider discovery and creation to issue create.
// ResolveProvider maps a provider name to its issue provider.
type IssueCreationEnvironment struct {
	ResolveProvider func(string) (port.IssueProvider, error)
	// StateRoot holds the issue create lifetime locks (holdIssueCreate).
	StateRoot string
}

// InferProvider infers the provider name from the remotes of repo.
func (e IssueCreationEnvironment) InferProvider(repo string) (string, error) {
	return inferProviderFromRepoRemotes(repo)
}

// ProjectAuthority resolves the project authority of repo for provider.
func (e IssueCreationEnvironment) ProjectAuthority(repo, provider string) (string, error) {
	return resolveProviderProjectAuthority(repo, provider)
}

// Resolve returns an invoker that creates issues through the named provider. A confirmed
// create runs under the issue-create lifetime lock and fails without invoking the provider
// when StateRoot is empty.
func (e IssueCreationEnvironment) Resolve(name string) (application.IssueCreateInvoker, error) {
	resolved, err := e.ResolveProvider(name)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, req port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
		if req.Confirm {
			if e.StateRoot == "" {
				return port.IssueProviderCreateIssueResult{OK: false}, &port.IssueProviderCreateError{Invoked: false, Err: fmt.Errorf("issue create needs the state root for its lifetime lock")}
			}
			held, release, err := holdIssueCreate(ctx, e.StateRoot, req.Repo, req.Title)
			if err != nil {
				return port.IssueProviderCreateIssueResult{OK: false}, err
			}
			defer release()
			ctx = held
		}
		return createRemoteIssueContext(ctx, req, resolved)
	}, nil
}

var _ application.IssueCreationEnvironment = IssueCreationEnvironment{}
