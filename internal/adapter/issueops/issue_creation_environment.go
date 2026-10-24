package issueops

import (
	"context"
	"fmt"

	application "issueops/internal/application/issueopsremote"
	"issueops/internal/port"
)

type IssueCreationEnvironment struct {
	ResolveProvider func(string) (port.IssueProvider, error)
	// StateRoot holds the issue create lifetime locks (HoldIssueCreate).
	StateRoot string
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
		if req.Confirm {
			if e.StateRoot == "" {
				return port.IssueProviderCreateIssueResult{OK: false}, &port.IssueProviderCreateError{Invoked: false, Err: fmt.Errorf("issue create needs the state root for its lifetime lock")}
			}
			held, release, err := HoldIssueCreate(ctx, e.StateRoot, req.Repo, req.Title)
			if err != nil {
				return port.IssueProviderCreateIssueResult{OK: false}, err
			}
			defer release()
			ctx = held
		}
		return CreateRemoteIssueContext(ctx, req, resolved)
	}, nil
}

var _ application.IssueCreationEnvironment = IssueCreationEnvironment{}
