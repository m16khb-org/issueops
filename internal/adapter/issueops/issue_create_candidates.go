package issueops

import (
	"context"
	"fmt"

	application "issueops/internal/application/issueopsremote"
	"issueops/internal/port"
)

type IssueCreateCandidateSource struct {
	Resolve func(string) (port.IssueProvider, error)
}

func (s IssueCreateCandidateSource) Find(ctx context.Context, provider string, req port.IssueProviderFindIssueCreateCandidatesRequest) (port.IssueProviderFindIssueCreateCandidatesResult, error) {
	resolved, err := s.Resolve(provider)
	if err != nil {
		return port.IssueProviderFindIssueCreateCandidatesResult{}, err
	}
	reconciler, ok := resolved.(port.IssueProviderIssueCreateReconciler)
	if !ok {
		return port.IssueProviderFindIssueCreateCandidatesResult{}, fmt.Errorf("provider %s does not support issue create reconciliation", provider)
	}
	return reconciler.FindIssueCreateCandidates(ctx, req)
}

var _ application.IssueCandidateSource = IssueCreateCandidateSource{}
